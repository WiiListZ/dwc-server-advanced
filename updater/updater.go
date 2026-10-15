// Package updater checks GitHub releases for new server versions and installs
// them by replacing the running executable and restarting the server.
package updater

import (
	"context"
	"crypto/sha256"
	"dwc-server-advanced/common"
	"dwc-server-advanced/logging"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const moduleName = "UPDATER"

// Mode says what the updater may do on this machine.
type Mode string

const (
	// ModeSelf downloads and installs releases.
	ModeSelf Mode = "self"
	// ModeNotify only reports new releases (containers, missing asset).
	ModeNotify Mode = "notify"
	// ModeDisabled turns the checker off.
	ModeDisabled Mode = "disabled"
)

// Release is the part of a GitHub release the updater needs.
type Release struct {
	Tag          string    `json:"tag"`
	Name         string    `json:"name"`
	Notes        string    `json:"notes"`
	URL          string    `json:"url"`
	Published    time.Time `json:"published"`
	AssetName    string    `json:"asset,omitempty"`
	assetURL     string
	checksumsURL string
}

// Schedule is a pending install chosen by the owner.
type Schedule struct {
	Version   string    `json:"version"`
	At        time.Time `json:"at"`
	Moderator string    `json:"moderator"`
}

// Store persists the schedule so it survives restarts.
type Store interface {
	LoadSchedule() (*Schedule, error)
	SaveSchedule(*Schedule) error
}

// Status is what the panel shows.
type Status struct {
	Current     string     `json:"current"`
	Latest      *Release   `json:"latest,omitempty"`
	Available   bool       `json:"available"`
	Mode        Mode       `json:"mode"`
	ModeReason  string     `json:"mode_reason,omitempty"`
	LastChecked *time.Time `json:"last_checked,omitempty"`
	Error       string     `json:"error,omitempty"`
	Schedule    *Schedule  `json:"schedule,omitempty"`
	Installing  bool       `json:"installing"`
	Platform    string     `json:"platform"`
}

var (
	mutex       sync.Mutex
	store       Store
	latest      *Release
	lastChecked *time.Time
	lastError   string
	schedule    *Schedule
	installing  bool
	httpClient  = &http.Client{Timeout: 2 * time.Minute}

	// restartFunc and executablePath are replaced in tests.
	restartFunc    = common.RequestRestart
	executablePath = os.Executable
)

// AssetName is the release asset for this platform, e.g.
// dwc-server-advanced_linux_amd64 or dwc-server-advanced_windows_amd64.exe.
func AssetName() string {
	name := fmt.Sprintf("dwc-server-advanced_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func currentMode() (Mode, string) {
	cfg := common.GetConfig().Updates
	if !cfg.Enabled {
		return ModeDisabled, "Updates are disabled in the config."
	}
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("DWC_UPDATE_MODE"))); v == "notify" {
		return ModeNotify, "Running in a container: pull the new image to update."
	}
	return ModeSelf, ""
}

// Start loads the saved schedule and runs the check and schedule loops.
func Start(s Store) {
	store = s
	cleanupOldExecutable()

	if saved, err := s.LoadSchedule(); err == nil && saved != nil {
		mutex.Lock()
		schedule = saved
		mutex.Unlock()
	}

	if mode, _ := currentMode(); mode == ModeDisabled {
		return
	}

	go func() {
		interval := time.Duration(common.GetConfig().Updates.CheckIntervalMinutes) * time.Minute
		for {
			if _, err := Check(); err != nil {
				logging.Warn(moduleName, "Release check failed:", err)
			}
			time.Sleep(interval)
		}
	}()

	go func() {
		for {
			time.Sleep(15 * time.Second)
			runDueSchedule()
		}
	}()
}

// GetStatus reports the current state without contacting GitHub.
func GetStatus() Status {
	mode, reason := currentMode()
	mutex.Lock()
	defer mutex.Unlock()

	status := Status{
		Current:     common.Version,
		Latest:      latest,
		Mode:        mode,
		ModeReason:  reason,
		LastChecked: lastChecked,
		Error:       lastError,
		Schedule:    schedule,
		Installing:  installing,
		Platform:    runtime.GOOS + "/" + runtime.GOARCH,
	}
	if latest != nil {
		status.Available = IsNewer(latest.Tag, common.Version)
		if status.Available && mode == ModeSelf && latest.assetURL == "" {
			status.Mode = ModeNotify
			status.ModeReason = "The release has no build for " + status.Platform + "."
		}
	}
	return status
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Check fetches the latest release from GitHub.
func Check() (Status, error) {
	cfg := common.GetConfig().Updates
	url := strings.TrimRight(cfg.APIBaseURL, "/") + "/repos/" + cfg.Repository + "/releases/latest"

	release, err := fetchRelease(url)
	now := time.Now().UTC()

	mutex.Lock()
	lastChecked = &now
	if err != nil {
		lastError = err.Error()
	} else {
		lastError = ""
		latest = release
	}
	mutex.Unlock()

	if err == nil && release != nil && IsNewer(release.Tag, common.Version) {
		logging.Notice(moduleName, "New release available:", release.Tag, "(running", common.Version+")")
	}
	return GetStatus(), err
}

func fetchRelease(url string) (*Release, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "dwc-server-advanced/"+common.Version)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No release published yet.
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub answered %s", resp.Status)
	}

	var gh githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&gh); err != nil {
		return nil, err
	}
	if gh.Draft || gh.Prerelease || gh.TagName == "" {
		return nil, nil
	}

	release := &Release{Tag: gh.TagName, Name: gh.Name, Notes: gh.Body, URL: gh.HTMLURL, Published: gh.PublishedAt}
	for _, asset := range gh.Assets {
		switch asset.Name {
		case AssetName():
			release.AssetName = asset.Name
			release.assetURL = asset.BrowserDownloadURL
		case "checksums.txt":
			release.checksumsURL = asset.BrowserDownloadURL
		}
	}
	return release, nil
}

// ScheduleInstall sets or replaces the pending install.
func ScheduleInstall(at time.Time, moderator string) (Schedule, error) {
	mutex.Lock()
	if latest == nil || !IsNewer(latest.Tag, common.Version) {
		mutex.Unlock()
		return Schedule{}, errors.New("no newer release to install")
	}
	next := Schedule{Version: latest.Tag, At: at.UTC(), Moderator: moderator}
	schedule = &next
	mutex.Unlock()

	if store != nil {
		if err := store.SaveSchedule(&next); err != nil {
			return next, err
		}
	}
	logging.Notice(moduleName, "Update to", next.Version, "scheduled for", next.At.Format(time.RFC3339), "by", moderator)
	return next, nil
}

// CancelSchedule removes the pending install.
func CancelSchedule() error {
	mutex.Lock()
	schedule = nil
	mutex.Unlock()
	if store != nil {
		return store.SaveSchedule(nil)
	}
	return nil
}

func runDueSchedule() {
	mutex.Lock()
	due := schedule != nil && !time.Now().Before(schedule.At) && !installing
	mutex.Unlock()
	if !due {
		return
	}

	if _, err := Check(); err != nil {
		logging.Warn(moduleName, "Release check before scheduled install failed:", err)
	}
	if err := Install(); err != nil {
		logging.Error(moduleName, "Scheduled update failed:", err)
		mutex.Lock()
		lastError = "Scheduled update failed: " + err.Error()
		mutex.Unlock()
	}
}

// Install downloads the latest release, replaces the executable and asks the
// frontend to restart the server. It returns before the restart happens.
func Install() error {
	if mode, reason := currentMode(); mode != ModeSelf {
		return errors.New(reason)
	}

	mutex.Lock()
	if installing {
		mutex.Unlock()
		return errors.New("an update is already being installed")
	}
	release := latest
	if release == nil || !IsNewer(release.Tag, common.Version) {
		mutex.Unlock()
		return errors.New("no newer release to install")
	}
	if release.assetURL == "" || release.checksumsURL == "" {
		mutex.Unlock()
		return fmt.Errorf("release %s has no %s or checksums.txt", release.Tag, AssetName())
	}
	installing = true
	mutex.Unlock()

	err := install(release)

	mutex.Lock()
	if err != nil {
		installing = false
		lastError = err.Error()
	} else {
		schedule = nil
	}
	mutex.Unlock()

	if err != nil {
		return err
	}
	if store != nil {
		_ = store.SaveSchedule(nil)
	}

	logging.Event("server_update_installing", map[string]any{"from": common.Version, "to": release.Tag})
	logging.Notice(moduleName, "Installed", release.Tag, "- restarting the server")
	go func() {
		// Leave time for the HTTP response and the event to be written.
		time.Sleep(time.Second)
		if err := restartFunc(); err != nil {
			logging.Error(moduleName, "Restart request failed, restart the server manually:", err)
		}
	}()
	return nil
}

func install(release *Release) error {
	expected, err := fetchChecksum(release.checksumsURL, AssetName())
	if err != nil {
		return err
	}

	exe, err := executablePath()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	tmp := exe + ".new"
	if err := download(release.assetURL, tmp, expected); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return replaceExecutable(exe, tmp)
}

func fetchChecksum(url, name string) (string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading checksums.txt: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s", name)
}

func download(url, path, expectedSHA256 string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: %s", filepath.Base(path), resp.Status)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("cannot write next to the executable: %w", err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, 512<<20))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}

	if got := hex.EncodeToString(hash.Sum(nil)); got != expectedSHA256 {
		return fmt.Errorf("checksum mismatch for the download (got %s, want %s)", got, expectedSHA256)
	}
	return nil
}

// replaceExecutable swaps in the new binary. Windows cannot overwrite a
// running executable but can rename it, so the old file is moved aside first.
func replaceExecutable(exe, next string) error {
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("cannot replace the executable: %w", err)
	}
	if err := os.Rename(next, exe); err != nil {
		_ = os.Rename(old, exe)
		return fmt.Errorf("cannot replace the executable: %w", err)
	}
	return nil
}

func cleanupOldExecutable() {
	if exe, err := executablePath(); err == nil {
		_ = os.Remove(exe + ".old")
		_ = os.Remove(exe + ".new")
	}
}

// IsNewer reports whether tag is a later version than current. Versions look
// like v1.2.3; anything else (such as "dev") counts as 0.0.0.
func IsNewer(tag, current string) bool {
	a, b := parseVersion(tag), parseVersion(current)
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func parseVersion(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	for i, part := range strings.SplitN(v, ".", 3) {
		n, err := strconv.Atoi(part)
		if err != nil {
			return [3]int{}
		}
		out[i] = n
	}
	return out
}
