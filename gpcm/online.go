package gpcm

import (
	"dwc-server-advanced/common"
	"sort"
)

type OnlineUserInfo struct {
	ProfileID       uint32 `json:"pid,omitempty"`
	FriendCode      string `json:"fc,omitempty"`
	Name            string `json:"name"`
	GameName        string `json:"game"`
	GameCode        string `json:"game_code"`
	Status          string `json:"status,omitempty"`
	RemoteAddr      string `json:"address,omitempty"`
	Restricted      bool   `json:"restricted,omitempty"`
	PayloadFeatures bool   `json:"payload_features,omitempty"`
}

type LoginMessageTargetInfo struct {
	Online          bool `json:"online"`
	PayloadFeatures bool `json:"payload_features"`
}

func GetOnlineUsers() []OnlineUserInfo {
	mutex.Lock()
	defer mutex.Unlock()

	users := []OnlineUserInfo{}
	for _, session := range sessions {
		if !session.LoggedIn {
			continue
		}

		name := session.InGameName
		if name == "" {
			name = session.User.LastInGameSn
		}

		friendCode := sessionFriendCode(session)

		users = append(users, OnlineUserInfo{
			ProfileID:       session.User.ProfileId,
			FriendCode:      friendCode,
			Name:            name,
			GameName:        session.GameName,
			GameCode:        session.GameCode,
			Status:          session.Status,
			RemoteAddr:      session.RemoteAddr,
			Restricted:      session.User.Restricted,
			PayloadFeatures: session.PayloadFeatures,
		})
	}

	sort.Slice(users, func(i, j int) bool {
		if users[i].Name == users[j].Name {
			return users[i].ProfileID < users[j].ProfileID
		}

		return users[i].Name < users[j].Name
	})

	return users
}

func GetLoginMessageTargetInfo(profileID uint32) LoginMessageTargetInfo {
	mutex.Lock()
	defer mutex.Unlock()

	session, exists := sessions[profileID]
	if !exists || !session.LoggedIn {
		return LoginMessageTargetInfo{}
	}

	return LoginMessageTargetInfo{
		Online:          true,
		PayloadFeatures: session.PayloadFeatures,
	}
}

func sessionFriendCode(session *GameSpySession) string {
	if len(session.User.GsbrCode) >= 4 {
		return common.CalcFriendCodeString(session.User.ProfileId, session.User.GsbrCode[:4])
	}
	if len(session.GameCode) >= 4 {
		return common.CalcFriendCodeString(session.User.ProfileId, session.GameCode[:4])
	}

	return ""
}
