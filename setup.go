package main

import (
	"dwc-server-advanced/api"
	"dwc-server-advanced/common"
	"fmt"
	"net"
	"os"
	"strings"
)

// runFirstSetupIfNeeded serves the setup wizard when there is no config file
// yet, and returns once the administrator has finished it.
func runFirstSetupIfNeeded() {
	if common.ConfigFileExists() {
		return
	}

	token := common.RandomString(24)
	addresses := []string{"0.0.0.0:80", "0.0.0.0:8080"}
	if custom := strings.TrimSpace(os.Getenv("DWC_SETUP_ADDRESS")); custom != "" {
		addresses = []string{custom}
	}

	var lastErr error
	for _, address := range addresses {
		port := address[strings.LastIndex(address, ":")+1:]
		if l, err := net.Listen("tcp", address); err != nil {
			lastErr = err
			continue
		} else {
			_ = l.Close()
		}

		fmt.Println()
		fmt.Println("  No config.yml found: first-run setup")
		fmt.Println("  Open one of these links in a browser to configure the server:")
		for _, host := range setupHosts(address) {
			if port == "80" {
				fmt.Printf("    http://%s/setup?token=%s\n", host, token)
			} else {
				fmt.Printf("    http://%s:%s/setup?token=%s\n", host, port, token)
			}
		}
		fmt.Println("  Keep the token private: anyone with it can configure the server.")
		fmt.Println()

		startDir, _ := os.Getwd()
		if err := api.RunSetup(api.SetupOptions{
			Address:      address,
			Token:        token,
			Schema:       embeddedSchema,
			DataDir:      startDir,
			DataDirFixed: dataDirExplicit,
		}); err != nil {
			lastErr = err
			continue
		}
		if wd, err := os.Getwd(); err == nil {
			fmt.Println("  Setup complete, starting the server in", wd)
		} else {
			fmt.Println("  Setup complete, starting the server")
		}
		return
	}

	fmt.Fprintln(os.Stderr, "Cannot start the setup wizard:", lastErr)
	fmt.Fprintln(os.Stderr, "Set DWC_SETUP_ADDRESS (for example 0.0.0.0:8080) or create config.yml from config_example.yml.")
	os.Exit(1)
}

// setupHosts lists the hosts the wizard can be reached on.
func setupHosts(address string) []string {
	host, _, _ := net.SplitHostPort(address)
	if host != "" && host != "0.0.0.0" && host != "::" {
		return []string{host}
	}

	hosts := []string{"localhost"}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() && !ipnet.IP.IsLinkLocalUnicast() {
				hosts = append(hosts, ipnet.IP.String())
			}
		}
	}
	return hosts
}
