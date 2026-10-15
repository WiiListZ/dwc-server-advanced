package common

// Version is the server version. Release builds set it from the git tag:
//
//	go build -ldflags "-X dwc-server-advanced/common.Version=v1.2.3"
var Version = "dev"
