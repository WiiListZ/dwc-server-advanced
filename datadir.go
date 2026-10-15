package main

import (
	"dwc-server-advanced/common"
	"embed"
	"fmt"
	"os"
	"strings"
)

// embeddedSchema creates the database tables during first-run setup.
//
//go:embed schema.sql
var embeddedSchema string

// embeddedDocs is the documentation shown in Panel → Server.
//
//go:embed README.md docs/*.md
var embeddedDocs embed.FS

// dataDirExplicit is true when --dir or DWC_HOME chose the data directory.
var dataDirExplicit bool

// chdirToDataDir moves into the data directory, which holds config.yml,
// certificates, payload/, state/ and logs/. It is chosen with --dir <path>
// (or --dir=<path>) or the DWC_HOME environment variable. Without them, a
// dwc-home.txt written by the setup wizard (in the current folder or next to
// the executable) is followed; otherwise the current directory is used. The
// flag is removed from the returned arguments.
func chdirToDataDir(args []string) []string {
	dir := strings.TrimSpace(os.Getenv("DWC_HOME"))
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--dir" && i+1 < len(args):
			dir = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--dir="):
			dir = strings.TrimPrefix(args[i], "--dir=")
		default:
			rest = append(rest, args[i])
		}
	}

	dataDirExplicit = dir != ""
	if dir == "" && !common.ConfigFileExists() {
		dir = common.ReadHomePointer(".")
		if dir == "" {
			dir = common.ReadHomePointer(common.ExecutableDir())
		}
	}

	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintln(os.Stderr, "Cannot use data directory", dir+":", err)
			os.Exit(1)
		}
	}

	// Child processes (the backend, or the server restarted after an update)
	// inherit the absolute data directory.
	if wd, err := os.Getwd(); err == nil {
		_ = os.Setenv("DWC_HOME", wd)
	}
	return rest
}
