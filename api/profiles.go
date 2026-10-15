package api

import (
	"bytes"
	"dwc-server-advanced/common"
	"dwc-server-advanced/database"
	"dwc-server-advanced/gpcm"
	"dwc-server-advanced/logging"
	"dwc-server-advanced/qr2"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	profileBioMaxLength   = 300
	profileAvatarMaxBytes = 2 << 20
	profileAvatarMaxSide  = 4096
	profileAvatarSize     = 256
)

var profilePageHTML = mustReadPage("profile.html")

func profilesEnabled() bool {
	return common.GetConfig().Profiles.Enabled
}

func loadProfiles() {
	if err := db.UpdateProfilesTable(); err != nil {
		panic(err)
	}
}

// profileAvatarURL is versioned so browsers can cache avatars forever.
func profileAvatarURL(profile database.SiteProfile) string {
	if !profile.HasAvatar {
		return ""
	}
	version := ""
	if profile.AvatarUpdated != nil {
		version = "?v=" + strconv.FormatInt(profile.AvatarUpdated.Unix(), 10)
	}
	return "/api/public/avatars/" + url.PathEscape(profile.Username) + version
}

type profileGameIdentity struct {
	Name       string `json:"name"`
	FriendCode string `json:"fc,omitempty"`
}

type profileRoomStatus struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	State    string `json:"state,omitempty"`
	Players  int    `json:"players"`
	IsHost   bool   `json:"host"`
	Region   string `json:"rk,omitempty"`
	GameName string `json:"game"`
}

type profileStatus struct {
	Online   bool               `json:"online"`
	Name     string             `json:"name,omitempty"`
	Game     string             `json:"game,omitempty"`
	GameCode string             `json:"game_code,omitempty"`
	Room     *profileRoomStatus `json:"room,omitempty"`
}

type publicProfileResponse struct {
	Username    string                 `json:"username"`
	Role        string                 `json:"role"`
	Created     time.Time              `json:"created"`
	Bio         string                 `json:"bio"`
	AvatarURL   string                 `json:"avatar_url,omitempty"`
	ShowStats   bool                   `json:"show_stats"`
	Linked      bool                   `json:"linked"`
	Status      profileStatus          `json:"status"`
	Profiles    []profileGameIdentity  `json:"profiles,omitempty"`
	Stats       *database.ProfileStats `json:"stats,omitempty"`
	CanModerate bool                   `json:"can_moderate,omitempty"`
	IsOwn       bool                   `json:"is_own,omitempty"`
}

// requireProfiles answers 404 when the profile system is turned off, so its
// routes look like they do not exist.
func requireProfiles(w http.ResponseWriter, r *http.Request) bool {
	if profilesEnabled() {
		return true
	}
	http.NotFound(w, r)
	return false
}

// HandleProfilePage serves /u/<username>.
func HandleProfilePage(w http.ResponseWriter, r *http.Request) {
	if !requireProfiles(w, r) {
		return
	}
	username := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/u/"), "/")
	if username == "" || strings.Contains(username, "/") {
		http.NotFound(w, r)
		return
	}
	if _, found, err := db.GetSiteProfile(username); err != nil || !found {
		http.NotFound(w, r)
		return
	}
	writeHTMLPage(w, r, profilePageHTML)
}

func canModerateProfiles(account database.PanelAccount) bool {
	return panelAccountHasRight(account, panelRightBan)
}

// HandlePublicProfile serves GET /api/public/profiles/<username>.
func HandlePublicProfile(w http.ResponseWriter, r *http.Request) {
	if !requireProfiles(w, r) || !allowMethod(w, r, http.MethodGet) {
		return
	}
	username := strings.TrimPrefix(r.URL.Path, "/api/public/profiles/")
	profile, found, err := db.GetSiteProfile(username)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}
	if !found {
		writePanelError(w, http.StatusNotFound, "profile not found")
		return
	}

	profileIDs, err := db.ListPanelAccountProfileIDs(profile.AccountID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	response := publicProfileResponse{
		Username:  profile.Username,
		Role:      profile.Role,
		Created:   profile.Created,
		Bio:       profile.Bio,
		AvatarURL: profileAvatarURL(profile),
		ShowStats: profile.ShowStats,
		Linked:    len(profileIDs) > 0,
		Status:    liveProfileStatus(profileIDs),
	}

	viewer, loggedIn := currentPanelAccount(r)
	if loggedIn {
		response.IsOwn = viewer.ID == profile.AccountID
		response.CanModerate = canModerateProfiles(viewer)
	}

	if profile.ShowStats || response.IsOwn {
		response.Profiles = profileIdentities(profile.AccountID)
		stats, err := db.GetProfileStats(profileIDs)
		if err != nil {
			logging.Error("API", "Failed to load profile stats:", err)
		} else {
			response.Stats = &stats
		}
	}

	writePanelJSON(w, http.StatusOK, response)
}

func profileIdentities(accountID int) []profileGameIdentity {
	identities := []profileGameIdentity{}
	for _, p := range db.AccountGameProfiles(accountID) {
		identities = append(identities, profileGameIdentity{Name: p.InGameName, FriendCode: p.FriendCode})
	}
	return identities
}

// liveProfileStatus reports the game and room of the first linked profile
// that is online right now.
func liveProfileStatus(profileIDs []uint32) profileStatus {
	if len(profileIDs) == 0 {
		return profileStatus{}
	}
	wanted := map[uint32]bool{}
	for _, profileID := range profileIDs {
		wanted[profileID] = true
	}

	status := profileStatus{}
	var onlineID uint32
	for _, user := range gpcm.GetOnlineUsers() {
		if wanted[user.ProfileID] {
			onlineID = user.ProfileID
			status = profileStatus{Online: true, Name: user.Name, Game: user.GameName, GameCode: user.GameCode}
			break
		}
	}
	if !status.Online {
		return status
	}

	pid := strconv.FormatUint(uint64(onlineID), 10)
	for _, room := range qr2.GetRooms(true) {
		for _, player := range room.Players {
			if player.ProfileID != pid {
				continue
			}
			status.Room = &profileRoomStatus{
				ID:       room.ID,
				Type:     room.MatchType,
				State:    room.State,
				Players:  len(room.Players),
				IsHost:   room.HostProfileID == pid,
				Region:   room.MKWRegion,
				GameName: room.GameName,
			}
			return status
		}
	}
	return status
}

// HandlePublicAvatar serves GET /api/public/avatars/<username> as PNG.
func HandlePublicAvatar(w http.ResponseWriter, r *http.Request) {
	if !requireProfiles(w, r) {
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	username := strings.TrimPrefix(r.URL.Path, "/api/public/avatars/")
	data, updated, found, err := db.GetProfileAvatar(username)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	if r.URL.Query().Get("v") != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=60")
	}
	http.ServeContent(w, r, "avatar.png", updated, bytes.NewReader(data))
}

type profileLink struct {
	FriendCode string `json:"fc"`
	Username   string `json:"username"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

// HandlePublicProfileLinks maps the friend codes of online players to the
// site profile they belong to, for linking names on the public pages.
func HandlePublicProfileLinks(w http.ResponseWriter, r *http.Request) {
	if !requireProfiles(w, r) || !allowMethod(w, r, http.MethodGet) {
		return
	}

	friendCodes := map[uint32]string{}
	profileIDs := []uint32{}
	for _, user := range gpcm.GetOnlineUsers() {
		if user.FriendCode == "" {
			continue
		}
		friendCodes[user.ProfileID] = user.FriendCode
		profileIDs = append(profileIDs, user.ProfileID)
	}

	usernames, err := db.ProfileUsernames(profileIDs)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to load profile links")
		return
	}

	profiles := map[string]database.SiteProfile{}
	links := []profileLink{}
	for profileID, username := range usernames {
		profile, cached := profiles[username]
		if !cached {
			profile, _, _ = db.GetSiteProfile(username)
			profiles[username] = profile
		}
		links = append(links, profileLink{
			FriendCode: friendCodes[profileID],
			Username:   username,
			AvatarURL:  profileAvatarURL(profile),
		})
	}

	writePanelJSON(w, http.StatusOK, links)
}

type myProfileResponse struct {
	Username  string `json:"username"`
	Bio       string `json:"bio"`
	AvatarURL string `json:"avatar_url,omitempty"`
	ShowStats bool   `json:"show_stats"`
}

func writeMyProfile(w http.ResponseWriter, accountID int) {
	profile, err := db.GetSiteProfileByAccount(accountID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}
	writePanelJSON(w, http.StatusOK, myProfileResponse{
		Username:  profile.Username,
		Bio:       profile.Bio,
		AvatarURL: profileAvatarURL(profile),
		ShowStats: profile.ShowStats,
	})
}

// HandlePanelMyProfile serves GET/PUT /api/panel/my/profile.
func HandlePanelMyProfile(w http.ResponseWriter, r *http.Request) {
	if !requireProfiles(w, r) {
		return
	}
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeMyProfile(w, account.ID)
	case http.MethodPut:
		var req struct {
			Bio       string `json:"bio"`
			ShowStats bool   `json:"show_stats"`
		}
		if !readPanelJSON(w, r, &req) {
			return
		}
		bio, err := cleanProfileBio(req.Bio)
		if err != nil {
			writePanelError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := db.SaveProfileSettings(account.ID, bio, req.ShowStats); err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to save profile")
			return
		}
		writeMyProfile(w, account.ID)
	default:
		w.Header().Set("Allow", "GET, PUT")
		writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// HandlePanelMyProfileAvatar serves POST/DELETE /api/panel/my/profile/avatar.
func HandlePanelMyProfileAvatar(w http.ResponseWriter, r *http.Request) {
	if !requireProfiles(w, r) {
		return
	}
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodPost:
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, profileAvatarMaxBytes))
		if err != nil {
			writePanelError(w, http.StatusRequestEntityTooLarge, "image must be 2 MB or smaller")
			return
		}
		avatar, err := processAvatar(data)
		if err != nil {
			writePanelError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := db.SetProfileAvatar(account.ID, avatar); err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to save avatar")
			return
		}
		writeMyProfile(w, account.ID)
	case http.MethodDelete:
		if err := db.SetProfileAvatar(account.ID, nil); err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to remove avatar")
			return
		}
		writeMyProfile(w, account.ID)
	default:
		w.Header().Set("Allow", "POST, DELETE")
		writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// HandlePanelProfileModeration lets moderators with the ban right remove an
// avatar or a bio from someone else's profile.
func HandlePanelProfileModeration(w http.ResponseWriter, r *http.Request) {
	if !requireProfiles(w, r) {
		return
	}
	moderator, ok := requirePanelRight(w, r, panelRightBan, "ban users")
	if !ok || !allowMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Username string `json:"username"`
		Clear    string `json:"clear"`
	}
	if !readPanelJSON(w, r, &req) {
		return
	}
	profile, found, err := db.GetSiteProfile(req.Username)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}
	if !found {
		writePanelError(w, http.StatusNotFound, "profile not found")
		return
	}
	if profile.Role == "owner" && moderator.Role != "owner" {
		writePanelError(w, http.StatusForbidden, "the owner profile cannot be moderated")
		return
	}

	switch req.Clear {
	case "avatar":
		err = db.SetProfileAvatar(profile.AccountID, nil)
	case "bio":
		err = db.ClearProfileBio(profile.AccountID)
	default:
		writePanelError(w, http.StatusBadRequest, "clear must be avatar or bio")
		return
	}
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	logging.Notice("API", "Moderator", moderator.Username, "cleared the", req.Clear, "of profile", profile.Username)
	writePanelJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func cleanProfileBio(bio string) (string, error) {
	bio = strings.TrimSpace(strings.ReplaceAll(bio, "\r\n", "\n"))
	if !utf8.ValidString(bio) {
		return "", errors.New("bio must be valid text")
	}
	if utf8.RuneCountInString(bio) > profileBioMaxLength {
		return "", errors.New("bio must be " + strconv.Itoa(profileBioMaxLength) + " characters or fewer")
	}
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, bio)
	for strings.Contains(cleaned, "\n\n\n") {
		cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
	}
	return cleaned, nil
}

// processAvatar decodes a PNG, JPEG or GIF, crops it to a centered square,
// scales it to profileAvatarSize and re-encodes it as PNG. Re-encoding drops
// metadata and anything that is not plain pixels.
func processAvatar(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("image required")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("image must be PNG, JPEG or GIF")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > profileAvatarMaxSide || config.Height > profileAvatarMaxSide {
		return nil, errors.New("image must be at most 4096x4096 pixels")
	}
	if format != "png" && format != "jpeg" && format != "gif" {
		return nil, errors.New("image must be PNG, JPEG or GIF")
	}

	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("image could not be read")
	}

	bounds := source.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	crop := image.Rect(0, 0, side, side).Add(image.Point{
		X: bounds.Min.X + (bounds.Dx()-side)/2,
		Y: bounds.Min.Y + (bounds.Dy()-side)/2,
	})
	square := image.NewNRGBA(image.Rect(0, 0, side, side))
	draw.Draw(square, square.Bounds(), source, crop.Min, draw.Src)

	output := scaleSquare(square, min(side, profileAvatarSize))
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, output); err != nil {
		return nil, errors.New("image could not be encoded")
	}
	return buffer.Bytes(), nil
}

// scaleSquare resizes a square image with a box filter (area average).
func scaleSquare(source *image.NRGBA, size int) *image.NRGBA {
	side := source.Bounds().Dx()
	if side == size {
		return source
	}
	output := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		y0, y1 := y*side/size, max((y+1)*side/size, y*side/size+1)
		for x := 0; x < size; x++ {
			x0, x1 := x*side/size, max((x+1)*side/size, x*side/size+1)
			var r, g, b, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					c := source.NRGBAAt(sx, sy)
					// Weight colours by alpha so transparent pixels do not darken edges.
					r += uint64(c.R) * uint64(c.A)
					g += uint64(c.G) * uint64(c.A)
					b += uint64(c.B) * uint64(c.A)
					a += uint64(c.A)
					n++
				}
			}
			if a == 0 {
				continue
			}
			output.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r / a),
				G: uint8(g / a),
				B: uint8(b / a),
				A: uint8(a / n),
			})
		}
	}
	return output
}
