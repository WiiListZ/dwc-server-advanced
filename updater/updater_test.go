package updater

import (
	"crypto/sha256"
	"dwc-server-advanced/common"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		tag, current string
		want         bool
	}{
		{"v1.2.0", "v1.1.9", true},
		{"v1.2.0", "v1.2.0", false},
		{"v1.10.0", "v1.9.0", true},
		{"v2.0.0", "v10.0.0", false},
		{"v1.0.0", "dev", true},
		{"v1.2.3-beta", "v1.2.2", true},
		{"garbage", "v1.0.0", false},
	}
	for _, c := range cases {
		if got := IsNewer(c.tag, c.current); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.tag, c.current, got, c.want)
		}
	}
}

// fakeGitHub serves a release with this platform's asset and checksums.txt.
func fakeGitHub(t *testing.T, tag string, binary []byte, checksum string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/repos/owner/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag, "name": tag, "body": "notes", "html_url": "https://example.test/" + tag,
			"published_at": time.Now().UTC().Format(time.RFC3339),
			"assets": []map[string]string{
				{"name": AssetName(), "browser_download_url": srv.URL + "/asset"},
				{"name": "checksums.txt", "browser_download_url": srv.URL + "/checksums"},
			},
		})
	})
	mux.HandleFunc("/asset", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(binary) })
	mux.HandleFunc("/checksums", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n%s  other_file\n", checksum, AssetName(), "00")
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func setup(t *testing.T, apiURL string) (exe string, restarted chan struct{}) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("updates:\n  enabled: true\n  repository: owner/repo\n  apiBaseUrl: "+apiURL+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WWFC_CONFIG", filepath.Join(dir, "config.yml"))
	common.ResetConfigForTest()

	exe = filepath.Join(dir, "server.bin")
	if err := os.WriteFile(exe, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	executablePath = func() (string, error) { return exe, nil }
	restarted = make(chan struct{}, 1)
	restartFunc = func() error { restarted <- struct{}{}; return nil }

	mutex.Lock()
	latest, schedule, installing, lastError, store = nil, nil, false, "", nil
	mutex.Unlock()
	common.Version = "v1.0.0"
	return exe, restarted
}

func TestCheckAndInstall(t *testing.T) {
	binary := []byte("new binary v1.1.0")
	sum := sha256.Sum256(binary)
	srv := fakeGitHub(t, "v1.1.0", binary, hex.EncodeToString(sum[:]))
	exe, restarted := setup(t, srv.URL)

	status, err := Check()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available || status.Latest == nil || status.Latest.Tag != "v1.1.0" || status.Mode != ModeSelf {
		t.Fatalf("unexpected status: %+v", status)
	}

	if err := Install(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(exe)
	if string(got) != string(binary) {
		t.Fatalf("executable not replaced, got %q", got)
	}
	if old, _ := os.ReadFile(exe + ".old"); string(old) != "old binary" {
		t.Fatalf("old executable not kept aside, got %q", old)
	}
	select {
	case <-restarted:
	case <-time.After(3 * time.Second):
		t.Fatal("restart was not requested")
	}
}

func TestInstallRejectsBadChecksum(t *testing.T) {
	srv := fakeGitHub(t, "v1.1.0", []byte("tampered"), "deadbeef")
	exe, restarted := setup(t, srv.URL)

	if _, err := Check(); err != nil {
		t.Fatal(err)
	}
	if err := Install(); err == nil {
		t.Fatal("install succeeded with a wrong checksum")
	}
	if got, _ := os.ReadFile(exe); string(got) != "old binary" {
		t.Fatalf("executable changed after a failed install: %q", got)
	}
	if _, err := os.Stat(exe + ".new"); !os.IsNotExist(err) {
		t.Fatal("partial download left behind")
	}
	select {
	case <-restarted:
		t.Fatal("restart requested after a failed install")
	case <-time.After(1500 * time.Millisecond):
	}
}

func TestScheduledInstallRuns(t *testing.T) {
	binary := []byte("scheduled binary")
	sum := sha256.Sum256(binary)
	srv := fakeGitHub(t, "v1.2.0", binary, hex.EncodeToString(sum[:]))
	exe, restarted := setup(t, srv.URL)

	if _, err := Check(); err != nil {
		t.Fatal(err)
	}
	if _, err := ScheduleInstall(time.Now().Add(time.Hour), "owner"); err != nil {
		t.Fatal(err)
	}
	runDueSchedule()
	if got, _ := os.ReadFile(exe); string(got) != "old binary" {
		t.Fatal("installed before the scheduled time")
	}

	mutex.Lock()
	schedule.At = time.Now().Add(-time.Second)
	mutex.Unlock()
	runDueSchedule()
	if got, _ := os.ReadFile(exe); string(got) != string(binary) {
		t.Fatalf("scheduled install did not run, got %q", got)
	}
	if GetStatus().Schedule != nil {
		t.Fatal("schedule not cleared after install")
	}
	select {
	case <-restarted:
	case <-time.After(3 * time.Second):
		t.Fatal("restart was not requested")
	}
}

func TestNoNewerRelease(t *testing.T) {
	srv := fakeGitHub(t, "v1.0.0", []byte("x"), "00")
	setup(t, srv.URL)
	status, err := Check()
	if err != nil {
		t.Fatal(err)
	}
	if status.Available {
		t.Fatal("same version reported as an update")
	}
	if err := Install(); err == nil {
		t.Fatal("install allowed without a newer release")
	}
}
