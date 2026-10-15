package common

import (
	"os"
	"path/filepath"
	"strings"
)

// HomePointerFile names the data directory chosen in the setup wizard. It is
// written next to the executable and in the folder the server was started
// from, so a later start without --dir finds the data directory again.
const HomePointerFile = "dwc-home.txt"

// ReadHomePointer returns the data directory named by a HomePointerFile in
// dir, or "" when there is none or it does not point to a directory.
func ReadHomePointer(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, HomePointerFile))
	if err != nil {
		return ""
	}
	target := strings.TrimSpace(string(data))
	if target == "" {
		return ""
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		return ""
	}
	return target
}

// WriteHomePointers records dataDir in each of dirs. Errors are ignored: a
// pointer that cannot be written only means --dir is needed on the next start.
func WriteHomePointers(dataDir string, dirs ...string) {
	seen := map[string]bool{}
	for _, dir := range dirs {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		if sameDir(dir, dataDir) {
			_ = os.Remove(filepath.Join(dir, HomePointerFile))
			continue
		}
		_ = os.WriteFile(filepath.Join(dir, HomePointerFile), []byte(dataDir+"\n"), 0o644)
	}
}

// ExecutableDir is the folder that contains the running binary.
func ExecutableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

func sameDir(a, b string) bool {
	ai, errA := os.Stat(a)
	bi, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ai, bi)
}
