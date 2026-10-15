package qr2

import (
	"bytes"
	"dwc-server-advanced/common"
	"dwc-server-advanced/logging"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/logrusorgru/aurora/v3"
	"gvisor.dev/gvisor/pkg/sleep"
)

func printHex(data []byte) string {
	logMsg := ""
	for i := 0; i < len(data); i++ {
		if (i % 32) == 0 {
			logMsg += "\n"
		}
		logMsg += fmt.Sprintf("%02x ", data[i])
	}

	return logMsg
}

type queuedServerMatchMessage struct {
	receiver *Session
	message  []byte
}

func buildServerMatchCommandMessageLocked(sender *Session, command byte, data common.MatchCommandData) ([]byte, bool) {
	if sender == nil || !sender.Authenticated {
		return nil, false
	}

	senderProfileID, err := strconv.ParseUint(sender.Data["dwc_pid"], 10, 32)
	if err != nil || senderProfileID == 0 {
		return nil, false
	}

	matchMessage, ok := common.EncodeMatchCommand(command, data)
	if !ok || len(matchMessage) > 0x80 {
		return nil, false
	}

	message := []byte{0xbb, 0x49, 0xcc, 0x4d}
	message = binary.LittleEndian.AppendUint32(message, uint32(data.Version))
	message = append(message, command, byte(len(matchMessage)))
	message = binary.LittleEndian.AppendUint16(message, uint16((sender.SearchID>>32)&0xffff))
	message = binary.BigEndian.AppendUint32(message, uint32(sender.SearchID&0xffffffff))
	message = binary.LittleEndian.AppendUint32(message, uint32(senderProfileID))
	message = append(message, matchMessage...)
	return message, true
}

func sendQueuedServerMatchMessages(messages []queuedServerMatchMessage) int {
	attempts := 0
	for _, queuedMessage := range messages {
		if queuedMessage.receiver == nil || len(queuedMessage.message) == 0 {
			continue
		}

		attempts++
		go sendServerClientMessage("QR2/MSG:server", queuedMessage.receiver, queuedMessage.message)
	}
	return attempts
}

func sendServerClientMessage(moduleName string, receiver *Session, message []byte) bool {
	if masterConn == nil {
		logging.Error(moduleName, "Cannot send match command before QR2 master socket is ready")
		return false
	}

	mutex.Lock()
	if receiver == nil || !receiver.Authenticated || receiver.login == nil {
		mutex.Unlock()
		return false
	}

	destPid := receiver.Data["dwc_pid"]
	if destPid == "" {
		destPid = "<UNKNOWN>"
	}
	destSessionID := receiver.SessionID
	packetCount := receiver.PacketCount + 1
	receiver.PacketCount = packetCount
	destAddr := receiver.Addr
	mutex.Unlock()

	payload := createResponseHeader(ClientMessageRequest, destSessionID)
	payload = append(payload, []byte{0, 0, 0, 0}...)
	binary.BigEndian.PutUint32(payload[len(payload)-4:], packetCount)
	payload = append(payload, message...)

	receiver.messageMutex.Lock()
	defer receiver.messageMutex.Unlock()

	if receiver.login == nil {
		return false
	}

	s := sleep.Sleeper{}
	defer s.Done()

	receiver.messageAckWaker.Clear()
	s.AddWaker(receiver.messageAckWaker)

	timeWaker := sleep.Waker{}
	s.AddWaker(&timeWaker)

	for timeOutCount := 0; timeOutCount < 3; timeOutCount++ {
		time.AfterFunc(1*time.Second, func() {
			timeWaker.Assert()
		})

		_, err := masterConn.WriteTo(payload, &destAddr)
		if err != nil {
			logging.Error(moduleName, "Error sending server match command:", err.Error())
			return false
		}
		logging.Notice(moduleName, "Sent server match command", aurora.Yellow(common.GetMatchCommandString(message[8])), "to", aurora.BrightCyan(destPid))

		switch s.Fetch(true) {
		case &timeWaker:
			continue
		default:
			return true
		}
	}

	logging.Warn(moduleName, "Timed out waiting for ack after server match command to", aurora.BrightCyan(destPid))
	return false
}

func SendClientMessage(senderIP string, destSearchID uint64, message []byte) {
	moduleName := "QR2/MSG"

	var matchData common.MatchCommandData
	var sender *Session
	var receiver *Session

	useSearchID := destSearchID < (1 << 24)
	mutex.Lock()
	if useSearchID {
		receiver = sessionBySearchID[destSearchID]
	} else {
		// It's an IP address, used in some circumstances
		receiver = sessions[destSearchID]
	}

	if receiver == nil || !receiver.Authenticated {
		mutex.Unlock()
		logging.Error(moduleName, "Destination", aurora.Cyan(destSearchID), "does not exist")
		return
	}

	if destPid, ok := receiver.Data["dwc_pid"]; !ok || destPid == "" {
		mutex.Unlock()
		logging.Error(moduleName, "Destination", aurora.Cyan(destSearchID), "has no profile ID")
		return
	}
	mutex.Unlock()

	login := receiver.login
	if login == nil || !login.DeviceAuthenticated {
		logging.Error(moduleName, "Destination", aurora.Cyan(destSearchID), "is not device authenticated")
		return
	}

	// Decode and validate the message
	isNatnegPacket := false
	if len(message) >= 2 && bytes.Equal(message[:2], []byte{0xfd, 0xfc}) {
		// Sending natneg cookie
		isNatnegPacket = true
		if len(message) != 0xA {
			logging.Error(moduleName, "Received invalid length NATNEG packet")
			return
		}

		natnegID := binary.LittleEndian.Uint32(message[0x6:0xA])
		moduleName = "QR2/MSG:s" + strconv.FormatUint(uint64(natnegID), 10)
	} else if len(message) >= 4 && (bytes.Equal(message[:4], []byte{0xbb, 0x49, 0xcc, 0x4d}) || bytes.Equal(message[:4], []byte("SBCM"))) {
		// DWC match command
		if len(message) < 0x14 || len(message) > 0x94 {
			logging.Error(moduleName, "Received invalid length match command packet")
			return
		}

		version := int(binary.LittleEndian.Uint32(message[0x04:0x08]))
		if version != 3 && version != 11 && version != 90 {
			logging.Error(moduleName, "Received invalid match version")
			return
		}

		senderProfileID := binary.LittleEndian.Uint32(message[0x10:0x14])
		moduleName = "QR2/MSG:p" + strconv.FormatUint(uint64(senderProfileID), 10)

		if (int(message[9]) + 0x14) != len(message) {
			logging.Error(moduleName, "Received invalid match command packet header")
			return
		}

		qr2IP := binary.BigEndian.Uint32(message[0x0C:0x10])
		qr2Port := binary.LittleEndian.Uint16(message[0x0A:0x0C])

		mutex.Lock()
		sender = sessions[(uint64(qr2Port)<<32)|uint64(qr2IP)]
		mutex.Unlock()
		if sender == nil || !sender.Authenticated {
			logging.Error(moduleName, "Session does not exist with QR2 IP and port")
			return
		}

		if !sender.setProfileID(moduleName, strconv.FormatUint(uint64(senderProfileID), 10), strings.Split(senderIP, ":")[0]) {
			// Error already logged
			return
		}

		login := sender.login
		if login == nil || !login.DeviceAuthenticated {
			logging.Error(moduleName, "Sender is not device authenticated")
			return
		}

		mutex.Lock()
		if sender.Data["gamename"] != receiver.Data["gamename"] {
			mutex.Unlock()
			logging.Error(moduleName, "Sender and receiver are not playing the same game")
			return
		}
		mutex.Unlock()

		var ok bool
		matchData, ok = common.DecodeMatchCommand(message[8], message[0x14:], version)
		if !ok {
			logging.Error(moduleName, "Received invalid match command:", aurora.Cyan(printHex(message)))
			return
		}

		if message[8] == common.MatchReservation {
			if matchData.Reservation.HasPublicIP {
				if qr2IP != matchData.Reservation.PublicIP {
					logging.Error(moduleName, "RESERVATION: Public IP mismatch in header and command")
					return
				}

				if qr2Port != matchData.Reservation.PublicPort {
					logging.Error(moduleName, "RESERVATION: Public port mismatch in header and command")
					return
				}
			}

			if version == 90 {
				if matchData.Reservation.LocalPort < 1024 {
					logging.Error(moduleName, "RESERVATION: Local port is reserved")
					return
				}
			}

			if useSearchID {
				matchData.Reservation.PublicIP = uint32(sender.SearchID & 0xffffffff)
				matchData.Reservation.PublicPort = uint16((sender.SearchID >> 32) & 0xffff)
				matchData.Reservation.LocalIP = 0
				matchData.Reservation.LocalPort = 0
			}
		}

		if message[8] == common.MatchResvOK {
			if qr2IP != matchData.ResvOK.PublicIP {
				logging.Error(moduleName, "RESERVATION: Public IP mismatch in header and command")
				return
			}

			if qr2Port != matchData.ResvOK.PublicPort {
				logging.Error(moduleName, "RESERVATION: Public port mismatch in header and command")
				return
			}

			if version == 90 {
				if matchData.ResvOK.LocalPort < 1024 {
					logging.Error(moduleName, "RESV_OK: Local port is reserved")
					return
				}

				if matchData.ResvOK.ProfileID != senderProfileID {
					logging.Error(moduleName, "RESV_OK: Profile ID mismatch in header")
					return
				}
			}

			if useSearchID {
				matchData.ResvOK.PublicIP = uint32(sender.SearchID & 0xffffffff)
				matchData.ResvOK.PublicPort = uint16((sender.SearchID >> 32) & 0xffff)
				matchData.ResvOK.LocalIP = 0
				matchData.ResvOK.LocalPort = 0
			}
		}

		if message[8] == common.MatchTellAddr {
			mutex.Lock()
			if sender.Data["publicip"] != receiver.Data["publicip"] {
				mutex.Unlock()
				logging.Error(moduleName, "TELL_ADDR: Public IP does not match receiver")
				return
			}
			mutex.Unlock()

			if matchData.TellAddr.LocalPort < 1024 {
				logging.Error(moduleName, "TELL_ADDR: Local port is reserved")
				return
			}
		}

		if useSearchID {
			// Convert public IP to search ID
			qr2SearchID := binary.LittleEndian.AppendUint16([]byte{}, uint16((sender.SearchID>>32)&0xffff))
			qr2SearchID = binary.BigEndian.AppendUint32(qr2SearchID, uint32(sender.SearchID&0xffffffff))
			message = append(message[:0x0A], append(qr2SearchID, message[0x10:0x14]...)...)
		} else {
			message = message[:0x14]
		}

		var matchMessage []byte
		matchMessage, ok = common.EncodeMatchCommand(message[8], matchData)
		if !ok || len(matchMessage) > 0x80 {
			logging.Error(moduleName, "Failed to reencode match command:", aurora.Cyan(printHex(message)))
			return
		}

		if len(matchMessage) != 0 {
			message = append(message, matchMessage...)
		}
	} else {
		logging.Error(moduleName, "Invalid message:", aurora.Cyan(printHex(message)))
		return
	}

	destSessionID, packetCount, destAddr := processClientMessage(moduleName, sender, receiver, message, isNatnegPacket, matchData)

	payload := createResponseHeader(ClientMessageRequest, destSessionID)

	payload = append(payload, []byte{0, 0, 0, 0}...)
	binary.BigEndian.PutUint32(payload[len(payload)-4:], packetCount)
	payload = append(payload, message...)

	receiver.messageMutex.Lock()
	defer receiver.messageMutex.Unlock()

	if receiver.login == nil {
		return
	}

	s := sleep.Sleeper{}
	defer s.Done()

	receiver.messageAckWaker.Clear()
	s.AddWaker(receiver.messageAckWaker)

	timeWaker := sleep.Waker{}
	s.AddWaker(&timeWaker)

	timeOutCount := 0
	for {
		time.AfterFunc(1*time.Second, func() {
			timeWaker.Assert()
		})

		_, err := masterConn.WriteTo(payload, &destAddr)
		if err != nil {
			logging.Error(moduleName, "Error sending message:", err.Error())
		}

		// Wait for an ack or timeout
		switch s.Fetch(true) {
		case &timeWaker:
			timeOutCount++

			// Enforce a 10 second timeout
			if timeOutCount <= 10 {
				break
			}

			logging.Error(moduleName, "Timed out waiting for ack")
			// Kick the player
			if login := receiver.login; login != nil {
				gpErrorCallback(login.ProfileID, "network_error")
				receiver.login = nil
			}
			return

		default:
			return
		}
	}
}

func processClientMessage(moduleName string, sender, receiver *Session, message []byte, isNatnegPacket bool, matchData common.MatchCommandData) (destSessionID uint32, packetCount uint32, destAddr net.UDPAddr) {
	mutex.Lock()

	destPid, ok := receiver.Data["dwc_pid"]
	if !ok || destPid == "" {
		destPid = "<UNKNOWN>"
	}

	destSessionID = receiver.SessionID
	packetCount = receiver.PacketCount + 1
	receiver.PacketCount = packetCount
	destAddr = receiver.Addr

	if isNatnegPacket {
		mutex.Unlock()
		cookie := binary.BigEndian.Uint32(message[0x6:0xA])
		logging.Notice(moduleName, "Send NN cookie", aurora.Cyan(strconv.FormatUint(uint64(cookie), 16)), "to", aurora.BrightCyan(destPid))
		return
	}
	defer mutex.Unlock()

	cmd := message[8]
	common.LogMatchCommand(moduleName, destPid, cmd, matchData)

	switch cmd {
	case common.MatchReservation:
		resvError := checkReservationAllowed(moduleName, sender, receiver, matchData.Reservation.MatchType)
		if resvError != "ok" {
			if resvError == "restricted" || resvError == "restricted_join" {
				logging.Error(moduleName, "RESERVATION: Restricted player attempted to join a public match")

				if sender.login != nil && sender.login.Restricted {
					profileId := sender.login.ProfileID

					mutex.Unlock()
					gpErrorCallback(profileId, resvError)
					mutex.Lock()
				}
				if receiver.login != nil && receiver.login.Restricted {
					profileId := receiver.login.ProfileID

					mutex.Unlock()
					gpErrorCallback(profileId, resvError)
					mutex.Lock()
				}
			}
			return
		}

		sender.Reservation = matchData
		sender.ReservationID = receiver.SearchID

	case common.MatchResvOK, common.MatchResvDeny, common.MatchResvWait:
		if receiver.ReservationID != sender.SearchID || receiver.Reservation.Reservation == nil {
			logging.Warn(moduleName, "Destination has no reservation with the sender")
			if receiver.groupPointer == nil || receiver.groupPointer != sender.groupPointer {
				return
			}
			// Allow the message through anyway to avoid a room deadlock
		}

		if receiver.Reservation.Version != matchData.Version {
			logging.Error(moduleName, "Reservation version mismatch")
			return
		}

		if cmd == common.MatchResvOK {
			if !processResvOK(moduleName, matchData.Version, *receiver.Reservation.Reservation, *matchData.ResvOK, sender, receiver) {
				return
			}
		} else if receiver.ReservationID == sender.SearchID {
			receiver.ReservationID = 0
		}

	case common.MatchTellAddr:
		processTellAddr(moduleName, sender, receiver)
	}

	return
}

func sendClientExploit(moduleName string, sessionCopy Session) {
	if len(sessionCopy.login.GameCode) != 4 || !common.IsUppercaseAlphanumeric(sessionCopy.login.GameCode) {
		logging.Error(moduleName, "Invalid game code:", aurora.Cyan(sessionCopy.login.GameCode))
		return
	}

	exploit, exploitPath, err := readClientExploit(sessionCopy.login.GameCode)
	if err != nil {
		logging.Error(moduleName, "Error reading exploit file", aurora.Cyan(sessionCopy.login.GameCode), "-", err.Error())
		return
	}
	logging.Info(moduleName, "Loaded SBCM exploit:", aurora.Cyan(exploitPath))

	mutex.Lock()
	session, sessionExists := sessions[makeLookupAddr(sessionCopy.Addr.String())]
	if !sessionExists {
		mutex.Unlock()
		logging.Error(moduleName, "Session not found")
		return
	}

	packetCount := session.PacketCount + 1
	session.PacketCount = packetCount
	mutex.Unlock()

	// Now send the exploit
	payload := createResponseHeader(ClientMessageRequest, sessionCopy.SessionID)
	payload = append(payload, []byte{0, 0, 0, 0}...)
	binary.BigEndian.PutUint32(payload[len(payload)-4:], packetCount)
	payload = append(payload, exploit[0xB:]...)

	go func() {
		for {
			_, err = masterConn.WriteTo(payload, &sessionCopy.Addr)
			if err != nil {
				logging.Error(moduleName, "Error sending message:", err.Error())
			}

			// Resend the message if no ack after 2 seconds
			time.Sleep(2 * time.Second)

			mutex.Lock()
			session, sessionExists := sessions[makeLookupAddr(sessionCopy.Addr.String())]
			if !sessionExists || session.ExploitReceived || session.login == nil || !session.login.NeedsExploit {
				mutex.Unlock()
				return
			}

			mutex.Unlock()
			logging.Notice(moduleName, "Resending SBCM exploit to DNS patcher client")
		}
	}()
}

func readClientExploit(gameCode string) ([]byte, string, error) {
	payloadDirs := []string{filepath.Join("payload", "sbcm")}
	if executable, err := os.Executable(); err == nil {
		executableDir := filepath.Join(filepath.Dir(executable), "payload", "sbcm")
		if executableDir != payloadDirs[0] {
			payloadDirs = append(payloadDirs, executableDir)
		}
	}

	var firstPath string
	var firstErr error
	for _, payloadDir := range payloadDirs {
		path := filepath.Join(payloadDir, "payload."+gameCode+".bin")
		exploit, err := os.ReadFile(path)
		if err == nil {
			if bytes.HasPrefix(exploit, []byte("WWFC/Payload")) {
				return nil, path, fmt.Errorf("%s is a stage-2 payload; expected the SBCM exploit from wfc-patcher-wii --exploit", path)
			}
			return exploit, path, nil
		}
		if firstErr == nil {
			firstPath = path
			firstErr = err
		}
		if !os.IsNotExist(err) {
			return nil, path, err
		}
	}

	return nil, firstPath, firstErr
}
