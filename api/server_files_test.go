package api

import (
	"dwc-server-advanced/common"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanServerPath(t *testing.T) {
	cases := map[string]string{
		"":                     ".",
		"/":                    ".",
		"payload":              "payload",
		"/payload/binary/":     filepath.Join("payload", "binary"),
		"payload\\binary":      filepath.Join("payload", "binary"),
		"../etc":               "etc",
		"payload/../../secret": "secret",
	}
	for input, want := range cases {
		got, ok := cleanServerPath(input)
		if !ok || got != want {
			t.Errorf("cleanServerPath(%q) = %q, %v; want %q", input, got, ok, want)
		}
	}
	for _, input := range []string{"C:/Windows", "file.txt:stream"} {
		if _, ok := cleanServerPath(input); ok {
			t.Errorf("cleanServerPath(%q) accepted", input)
		}
	}
}

func TestServerRootBlocksEscapes(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Skip("symlinks not available:", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := root.Open(filepath.Join("link", "secret.txt")); err == nil {
		t.Fatal("os.Root followed a symlink out of the server folder")
	}
}

func TestSetupFolder(t *testing.T) {
	start := t.TempDir()
	opts := SetupOptions{DataDir: start}

	if got := resolveSetupFolder(opts, ""); got != start {
		t.Fatalf("empty path = %q, want the start folder", got)
	}
	target := resolveSetupFolder(opts, "server-data")
	if target != filepath.Join(start, "server-data") {
		t.Fatalf("relative path = %q", target)
	}
	if status := prepareSetupFolder(target, false); status.Exists {
		t.Fatal("folder reported before it was created")
	}
	status := prepareSetupFolder(target, true)
	if !status.Exists || !status.Writable || status.HasConfig || status.Error != "" {
		t.Fatalf("prepareSetupFolder() = %+v", status)
	}
	if err := os.WriteFile(filepath.Join(target, "config.yml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !prepareSetupFolder(target, false).HasConfig {
		t.Fatal("existing config.yml not reported")
	}

	fixed := SetupOptions{DataDir: start, DataDirFixed: true}
	if got := resolveSetupFolder(fixed, "elsewhere"); got != start {
		t.Fatalf("fixed data dir moved to %q", got)
	}
}

func TestHomePointers(t *testing.T) {
	start := t.TempDir()
	data := t.TempDir()
	common.WriteHomePointers(data, start)
	if got := common.ReadHomePointer(start); got != data {
		t.Fatalf("ReadHomePointer() = %q, want %q", got, data)
	}
	// A pointer to the folder itself is pointless and is removed.
	common.WriteHomePointers(start, start)
	if got := common.ReadHomePointer(start); got != "" {
		t.Fatalf("pointer to itself kept: %q", got)
	}
}
