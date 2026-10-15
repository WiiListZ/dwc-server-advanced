package qr2

import (
	"dwc-server-advanced/common"
	"encoding/gob"
	"os"
	"strconv"
)

type LoginInfo struct {
	ProfileID           uint32
	GameCode            string
	InGameName          string
	ConsoleFriendCode   uint64
	FriendKeyGame       string
	GPPublicIP          string
	NeedsExploit        bool
	DeviceAuthenticated bool
	PayloadFeatures     bool
	Restricted          bool
	OpenHost            bool
	OpenHostMode        string
	OpenHostList        []uint32
	session             *Session
}

// IsOpenHostFor returns true when the auto-friend ("Open Host") behavior
// applies for the given peer profile ID under the current mode + list.
func (l *LoginInfo) IsOpenHostFor(peerProfileID uint32) bool {
	switch l.OpenHostMode {
	case "disabled":
		return false
	case "whitelist":
		for _, id := range l.OpenHostList {
			if id == peerProfileID {
				return true
			}
		}
		return false
	case "blacklist", "":
		for _, id := range l.OpenHostList {
			if id == peerProfileID {
				return false
			}
		}
		return l.OpenHost
	}
	return false
}

var logins = map[uint32]*LoginInfo{}

func Login(profileID uint32, gameCode string, inGameName string, consoleFriendCode uint64, fcGame string, publicIP string, needsExploit bool, deviceAuthenticated bool, payloadFeatures bool, restricted bool, openHost bool, openHostMode string, openHostList []uint32) {
	mutex.Lock()
	defer mutex.Unlock()

	if openHostMode == "" {
		openHostMode = "blacklist"
	}

	logins[profileID] = &LoginInfo{
		ProfileID:           profileID,
		GameCode:            gameCode,
		InGameName:          inGameName,
		ConsoleFriendCode:   consoleFriendCode,
		FriendKeyGame:       fcGame,
		GPPublicIP:          publicIP,
		NeedsExploit:        needsExploit,
		DeviceAuthenticated: deviceAuthenticated,
		PayloadFeatures:     payloadFeatures,
		Restricted:          restricted,
		OpenHost:            openHost,
		OpenHostMode:        openHostMode,
		OpenHostList:        append([]uint32(nil), openHostList...),
		session:             nil,
	}
}

func SetOpenHost(profileID uint32, openHost bool) {
	mutex.Lock()
	defer mutex.Unlock()

	if login, exists := logins[profileID]; exists {
		login.OpenHost = openHost
		if openHost {
			if login.OpenHostMode == "disabled" || login.OpenHostMode == "" {
				login.OpenHostMode = "blacklist"
			}
		} else {
			login.OpenHostMode = "disabled"
		}
	}
}

func SetOpenHostMode(profileID uint32, mode string, list []uint32) {
	mutex.Lock()
	defer mutex.Unlock()

	if mode == "" {
		mode = "blacklist"
	}

	if login, exists := logins[profileID]; exists {
		login.OpenHostMode = mode
		login.OpenHostList = append([]uint32(nil), list...)
		login.OpenHost = mode != "disabled"
	}
}

func SetDeviceAuthenticated(profileID uint32) {
	mutex.Lock()
	defer mutex.Unlock()

	if login, exists := logins[profileID]; exists {
		login.DeviceAuthenticated = true
		if login.session != nil {
			login.session.Data["+deviceauth"] = "1"
		}
	}
}

func Logout(profileID uint32) {
	mutex.Lock()
	defer mutex.Unlock()

	// Delete login's session
	if login, exists := logins[profileID]; exists {
		if login.session != nil {
			removeSession(makeLookupAddr(login.session.Addr.String()))
		}
	}

	delete(logins, profileID)
}

// Save logins to a file. Expects the mutex to be locked.
func saveLogins() error {
	file, err := os.OpenFile("state/qr2_logins.gob", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() {
		common.ShouldNotError(file.Close())
	}()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(logins)
}

// Load logins from a file. Expects the mutex to be locked, and the sessions to already be loaded.
func loadLogins() error {
	file, err := os.Open("state/qr2_logins.gob")
	if err != nil {
		return err
	}
	defer func() {
		common.ShouldNotError(file.Close())
	}()

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&logins)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		dwcPid := session.Data["dwc_pid"]
		if dwcPid == "" {
			continue
		}

		profileID, err := strconv.ParseUint(dwcPid, 10, 32)
		if err != nil {
			continue
		}

		if login, exists := logins[uint32(profileID)]; exists {
			login.session = session
			session.login = login
		}
	}

	return nil
}
