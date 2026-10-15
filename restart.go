package main

import (
	"dwc-server-advanced/logging"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/logrusorgru/aurora/v3"
)

// restartExitCode is used under systemd, where the unit restarts the server
// (Restart=always or Restart=on-failure).
const restartExitCode = 75

// RPCFrontendPacket.RestartForUpdate is called by the backend after it
// installed a new executable.
func (r *RPCFrontendPacket) RestartForUpdate(_ struct{}, _ *struct{}) error {
	go restartForUpdate()
	return nil
}

func restartForUpdate() {
	// Let the RPC reply reach the backend first.
	time.Sleep(300 * time.Millisecond)
	logging.Notice("FRONTEND", "Restarting to load the updated server")

	if rpcClient != nil {
		// The backend exits without replying, so the error is expected.
		_ = rpcClient.Call("RPCPacket.Shutdown", "", nil)
	}

	if os.Getenv("INVOCATION_ID") != "" {
		logging.Notice("FRONTEND", "Running under systemd, exiting so the service restarts")
		os.Exit(restartExitCode)
	}

	exe, err := os.Executable()
	if err != nil {
		logging.Error("FRONTEND", "Cannot find the executable to restart:", err)
		os.Exit(1)
	}

	cmd := exec.Command(exe, launchArgs...)
	cmd.Env = append(os.Environ(), "DWC_RESTARTED=1")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		logging.Error("FRONTEND", "Cannot start the updated server, start it manually:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// waitForRestartedPort waits until the previous process released the
// frontend RPC port, which it does only when it has fully exited.
func waitForRestartedPort(address string) {
	if os.Getenv("DWC_RESTARTED") == "" {
		return
	}
	_ = os.Unsetenv("DWC_RESTARTED")

	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		l, err := net.Listen("tcp", address)
		if err == nil {
			_ = l.Close()
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	logging.Warn("FRONTEND", "Previous process still holds", aurora.BrightCyan(address))
}
