package common

import (
	"dwc-server-advanced/logging"
	"errors"
	"net/rpc"
	"time"
)

var rpcFrontend *rpc.Client

type RPCFrontendPacket struct {
	Server string
	Index  uint64
	Data   []byte
}

// ConnectFrontend connects to the frontend RPC server
func ConnectFrontend() {
	config := GetConfig()

	var err error
	for i := 0; rpcFrontend == nil; i++ {
		rpcFrontend, err = rpc.Dial("tcp", config.BackendFrontendAddress)
		if err != nil {
			if i > 20 {
				panic(err)
			}

			// Sleep for 200 ms before trying again
			<-time.After(200 * time.Millisecond)
		}
	}
}

// SendPacket is used by backend servers to send a packet to a connection
func SendPacket(server string, index uint64, data []byte) error {
	if rpcFrontend == nil {
		ConnectFrontend()
	}

	err := rpcFrontend.Call("RPCFrontendPacket.SendPacket", RPCFrontendPacket{Server: server, Index: index, Data: data}, nil)
	if err != nil {
		logging.Error("COMMON", "Failed to send packet to frontend:", err)
	}
	return err
}

// CloseConnection is used by backend servers to close a connection
func CloseConnection(server string, index uint64) error {
	if rpcFrontend == nil {
		ConnectFrontend()
	}

	err := rpcFrontend.Call("RPCFrontendPacket.CloseConnection", RPCFrontendPacket{Server: server, Index: index}, nil)
	if err != nil {
		logging.Error("COMMON", "Failed to close connection:", err)
	}
	return err
}

// Ready will notify the frontend that the backend is ready to accept connections
func Ready() error {
	if rpcFrontend == nil {
		ConnectFrontend()
	}

	err := rpcFrontend.Call("RPCFrontendPacket.Ready", struct{}{}, nil)
	if err != nil {
		logging.Error("COMMON", "Failed to notify frontend that backend is ready:", err)
	}
	return err
}

// Shutdown will notify the frontend that the backend is shutting down
func Shutdown() (string, error) {
	if rpcFrontend == nil {
		ConnectFrontend()
	}

	var stateUuid string
	err := rpcFrontend.Call("RPCFrontendPacket.ShutdownBackend", struct{}{}, &stateUuid)
	if err != nil {
		logging.Error("COMMON", "Failed to notify frontend that backend is shutting down:", err)
	}

	return stateUuid, err
}

// VerifyState will verify the state UUID with the frontend
func VerifyState(stateUuid string) (bool, error) {
	if rpcFrontend == nil {
		ConnectFrontend()
	}

	valid := false
	err := rpcFrontend.Call("RPCFrontendPacket.VerifyState", stateUuid, &valid)
	if err != nil {
		logging.Error("COMMON", "Failed to verify state UUID with frontend:", err)
	}
	return valid, err
}

// ConsoleLines reads the console output collected by the frontend process,
// which includes the backend's output.
func ConsoleLines(since uint64) (logging.ConsoleSnapshot, error) {
	if rpcFrontend == nil {
		return logging.ConsoleSnapshot{}, errors.New("frontend not connected")
	}
	reply := logging.ConsoleSnapshot{}
	call := rpcFrontend.Go("RPCFrontendPacket.ConsoleLines", since, &reply, nil)
	select {
	case <-call.Done:
		return reply, call.Error
	case <-time.After(3 * time.Second):
		return logging.ConsoleSnapshot{}, errors.New("frontend did not answer")
	}
}

// RequestRestart asks the frontend to restart the whole server, which loads
// a freshly installed executable.
func RequestRestart() error {
	if rpcFrontend == nil {
		ConnectFrontend()
	}

	return rpcFrontend.Call("RPCFrontendPacket.RestartForUpdate", struct{}{}, nil)
}
