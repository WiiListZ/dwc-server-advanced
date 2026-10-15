package gpcm

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/logging"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

const (
	LangJapanese    = 0x00
	LangEnglish     = 0x01
	LangGerman      = 0x02
	LangFrench      = 0x03
	LangSpanish     = 0x04
	LangItalian     = 0x05
	LangDutch       = 0x06
	LangSimpChinese = 0x07
	LangTradChinese = 0x08
	LangKorean      = 0x09

	LangEnglishEU = 0x81
	LangFrenchEU  = 0x83
	LangSpanishEU = 0x84
)

const maxWWFCErrorMessageUTF16Units = 190

type WWFCErrorMessage struct {
	ErrorCode  int
	MessageRMC map[byte]string
}

type GPError struct {
	ErrorCode   int
	ErrorString string
	Fatal       bool
	WWFCMessage WWFCErrorMessage
	Reason      string
}

func MakeGPError(errorCode int, errorString string, fatal bool) GPError {
	return GPError{
		ErrorCode:   errorCode,
		ErrorString: errorString,
		Fatal:       fatal,
	}
}

var (
	ErrNone = MakeGPError(0xFFFF, "No error.", false)

	// General errors
	ErrGeneral          = MakeGPError(0x0000, "There was an unknown error.", true)
	ErrParse            = MakeGPError(0x0001, "There was an error parsing an incoming request.", true)
	ErrNotLoggedIn      = MakeGPError(0x0002, "This request cannot be processed because you are not logged in.", true)
	ErrBadSessionKey    = MakeGPError(0x0003, "This request cannot be processed because the session key is invalid.", true)
	ErrDatabase         = MakeGPError(0x0004, "This request cannot be processed because of a database error.", true)
	ErrNetwork          = MakeGPError(0x0005, "There was an error connecting a network socket.", true)
	ErrForcedDisconnect = MakeGPError(0x0006, "This profile has been disconnected by another login.", true)
	ErrConnectionClosed = MakeGPError(0x0007, "The server has closed the connection.", true)
	ErrUDPLayer         = MakeGPError(0x0008, "There was a problem with the UDP layer.", true)

	// Login errors
	ErrLogin                   = MakeGPError(0x0100, "There was an error logging in to the GP backend.", true)
	ErrLoginTimeout            = MakeGPError(0x0101, "The login attempt timed out.", true)
	ErrLoginBadNickname        = MakeGPError(0x0102, "The nickname provided was incorrect.", true)
	ErrLoginBadEmail           = MakeGPError(0x0103, "The email address provided was incorrect.", true)
	ErrLoginBadPassword        = MakeGPError(0x0104, "The password provided is incorrect.", true)
	ErrLoginBadProfile         = MakeGPError(0x0105, "The profile provided was incorrect.", true)
	ErrLoginProfileDeleted     = MakeGPError(0x0106, "The profile has been deleted.", true)
	ErrLoginConnectionFailed   = MakeGPError(0x0107, "The server has refused the connection.", true)
	ErrLoginServerAuthFailed   = MakeGPError(0x0108, "The server could not be authenticated.", true)
	ErrLoginBadUniqueNick      = MakeGPError(0x0109, "The uniquenick provided is incorrect.", true)
	ErrLoginBadPreAuth         = MakeGPError(0x010A, "There was an error validating the pre-authentication.", true)
	ErrLoginLoginTicketInvalid = MakeGPError(0x010B, "The login ticket was unable to be validated.", true)
	ErrLoginLoginTicketExpired = MakeGPError(0x010C, "The login ticket had expired and could not be used.", true)

	// New user errors
	ErrNewUser                  = MakeGPError(0x0200, "There was an error creating a new user.", true)
	ErrNewUserBadNickname       = MakeGPError(0x0201, "A profile with that nick already exists.", true)
	ErrNewUserBadPassword       = MakeGPError(0x0202, "The password does not match the email address.", true)
	ErrNewUserUniqueNickInvalid = MakeGPError(0x0203, "The uniquenick is invalid.", true)
	ErrNewUserUniqueNickInUse   = MakeGPError(0x0204, "The uniquenick is already in use.", true)

	// Update user information errors
	ErrUpdateUserInfo         = MakeGPError(0x0300, "There was an error updating the user information.", false)
	ErrUpdateUserInfoBadEmail = MakeGPError(0x0301, "A user with the email address provided already exists.", false)

	// New profile errors
	ErrNewProfile               = MakeGPError(0x0400, "There was an error creating a new profile.", false)
	ErrNewProfileBadNickname    = MakeGPError(0x0401, "The nickname to be replaced does not exist.", false)
	ErrNewProfileBadOldNickname = MakeGPError(0x0402, "A profile with the nickname provided already exists.", false)

	// Update profile errors
	ErrUpdateProfile            = MakeGPError(0x0500, "There was an error updating the profile information.", false)
	ErrUpdateProfileBadNickname = MakeGPError(0x0501, "A user with the nickname provided already exists.", false)

	// Add friend errors
	ErrAddFriend               = MakeGPError(0x0600, "There was an error adding a buddy.", false)
	ErrAddFriendBadFrom        = MakeGPError(0x0601, "The profile requesting to add a buddy is invalid.", false)
	ErrAddFriendBadNew         = MakeGPError(0x0602, "The profile requested is invalid.", false)
	ErrAddFriendAlreadyFriends = MakeGPError(0x0603, "The profile requested is already a buddy.", false)
	ErrAddFriendLocalBlock     = MakeGPError(0x0604, "The profile requested is on the local profile's block list.", false)
	ErrAddFriendBlocked        = MakeGPError(0x0605, "The profile requested is blocking you.", false)

	// Auth add friend errors
	ErrAuthAdd             = MakeGPError(0x0700, "There was an error authorizing an add buddy request.", false)
	ErrAuthAddBadFrom      = MakeGPError(0x0701, "The profile being authorized is invalid.", false)
	ErrAuthAddBadSignature = MakeGPError(0x0702, "The signature for the authorization is invalid.", false)
	ErrAuthAddLocalBlock   = MakeGPError(0x0703, "The profile requesting authorization is on a block list.", false)
	ErrAuthAddBlocked      = MakeGPError(0x0704, "The profile requested is blocking you.", false)

	// Status errors
	ErrStatus = MakeGPError(0x0800, "There was an error with the status string.", false)

	// Friend message errors
	ErrMessage                    = MakeGPError(0x0900, "There was an error sending a buddy message.", false)
	ErrMessageNotFriends          = MakeGPError(0x0901, "The profile the message was to be sent to is not a buddy.", false)
	ErrMessageExtInfoNotSupported = MakeGPError(0x0902, "The profile does not support extended info keys.", false)
	ErrMessageFriendOffline       = MakeGPError(0x0903, "The buddy to send a message to is offline.", false)

	// Get profile errors
	ErrGetProfile           = MakeGPError(0x0A00, "There was an error getting profile info.", false)
	ErrGetProfileBadProfile = MakeGPError(0x0A01, "The profile info was requested on is invalid.", false)

	// Delete friend errors
	ErrDeleteFriend           = MakeGPError(0x0B00, "There was an error deleting the buddy.", false)
	ErrDeleteFriendNotFriends = MakeGPError(0x0B01, "The buddy to be deleted is not a buddy.", false)

	// Delete profile errors
	ErrDeleteProfile            = MakeGPError(0x0C00, "There was an error deleting the profile.", false)
	ErrDeleteProfileLastProfile = MakeGPError(0x0C01, "The last profile cannot be deleted.", false)

	// Profile searching errors
	ErrSearch              = MakeGPError(0x0D00, "There was an error searching for a profile.", false)
	ErrSearchConnectFailed = MakeGPError(0x0D01, "The search attempt failed to connect to the server.", false)
	ErrSearchTimedOut      = MakeGPError(0x0D02, "The search did not return in a timely fashion.", false)

	// User check errors
	ErrCheck            = MakeGPError(0x0E00, "There was an error checking the user account.", false)
	ErrCheckBadEmail    = MakeGPError(0x0E01, "No account exists with the provided email address.", false)
	ErrCheckBadNickname = MakeGPError(0x0E02, "No such profile exists for the provided email address.", false)
	ErrCheckBadPassword = MakeGPError(0x0E03, "The password is incorrect.", false)

	// Revoke buddy errors
	ErrRevoke           = MakeGPError(0x0F00, "There was an error revoking the buddy.", false)
	ErrRevokeNotFriends = MakeGPError(0x0F01, "You are not a buddy of the profile.", false)

	// Register unique nick errors
	ErrRegisterUniqueNick             = MakeGPError(0x1000, "There was an error registering the uniquenick.", false)
	ErrRegisterUniqueNickTaken        = MakeGPError(0x1001, "The uniquenick is already taken.", false)
	ErrRegisterUniqueNickReserved     = MakeGPError(0x1002, "The uniquenick is reserved.", false)
	ErrRegisterUniqueNickBadNamespace = MakeGPError(0x1003, "Tried to register a nick with no namespace set.", false)

	// Register cdkey errors
	ErrRegisterCDKey             = MakeGPError(0x1100, "There was an error registering the cdkey.", false)
	ErrRegisterCDKeyBadKey       = MakeGPError(0x1101, "The cdkey is invalid.", false)
	ErrRegisterCDKeyAlreadySet   = MakeGPError(0x1102, "The profile has already been registered with a different cdkey.", false)
	ErrRegisterCDKeyAlreadyTaken = MakeGPError(0x1103, "The cdkey has already been registered to another profile.", false)

	// Add to block list errors
	ErrAddBlock               = MakeGPError(0x1200, "There was an error adding the player to the blocked list.", false)
	ErrAddBlockAlreadyBlocked = MakeGPError(0x1201, "The profile specified is already blocked.", false)

	// Remove from block list errors
	ErrRemoveBlock           = MakeGPError(0x1300, "There was an error removing the player from the blocked list.", false)
	ErrRemoveBlockNotBlocked = MakeGPError(0x1301, "The profile specified was not a member of the blocked list.", false)
)

var (
	// WWFC errors with custom messages
	WWFCMsgUnknownLoginError = WWFCErrorMessage{
		ErrorCode: 22000,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"An unknown error has occurred\n" +
				"while logging in to %[4]s.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgDolphinSetupRequired = WWFCErrorMessage{
		ErrorCode: 22001,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"Additional setup is required\n" +
				"to use %[4]s on Dolphin.\n" +
				"Check your server setup.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgProfileBannedTOS = WWFCErrorMessage{
		ErrorCode: 22002,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"You are banned from %[4]s\n" +
				"due to a violation of the\n" +
				"Terms of Service.\n" +
				"\n" +
				"Error Code: %[1]d\n",
		},
	}

	WWFCMsgProfileBannedTOSNow = WWFCErrorMessage{
		ErrorCode: 22002,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"You are banned from %[4]s\n" +
				"due to a violation of the\n" +
				"Terms of Service.\n" +
				"\n" +
				"Error Code: %[1]d\n",
		},
	}

	WWFCMsgProfileRestricted = WWFCErrorMessage{
		ErrorCode: 22003,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"You are banned from public\n" +
				"matches due to a violation\n" +
				"of the %[4]s rules.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgProfileRestrictedNow = WWFCErrorMessage{
		ErrorCode: 22003,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"You have been banned from public\n" +
				"matches due to a violation\n" +
				"of the %[4]s rules.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgProfileRestrictedCustom = WWFCErrorMessage{
		ErrorCode: 22002,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"You are banned from public matches.\n" +
				"Reason: %[3]s\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgProfileRestrictedNowCustom = WWFCErrorMessage{
		ErrorCode: 22002,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"You have been banned from public matches.\n" +
				"Reason: %[3]s\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgKickedGeneric = WWFCErrorMessage{
		ErrorCode: 22004,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"You have been kicked from\n" +
				"%[4]s.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgKickedModerator = WWFCErrorMessage{
		ErrorCode: 22004,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"You have been kicked from\n" +
				"%[4]s by a moderator.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgKickedRoomHost = WWFCErrorMessage{
		ErrorCode: 22004,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"You have been kicked from the\n" +
				"friend room by the room creator.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgKickedRoomHostCustom = WWFCErrorMessage{
		ErrorCode: 22004,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"You have been kicked from the\n" +
				"private room by the room creator.\n" +
				"Reason: %[3]s\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgKickedCustom = WWFCErrorMessage{
		ErrorCode: 22002,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"You have been kicked from\n" +
				"%[4]s by a moderator.\n" +
				"Reason: %[3]s\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgConsoleMismatch = WWFCErrorMessage{
		ErrorCode: 22005,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"The console you are using is not\n" +
				"the device used to register this\n" +
				"profile.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgConsoleMismatchDolphin = WWFCErrorMessage{
		ErrorCode: 22005,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"The console you are using is not\n" +
				"the device used to register this\n" +
				"profile. Please make sure you've\n" +
				"set up your NAND correctly.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgProfileIDInvalid = WWFCErrorMessage{
		ErrorCode: 22006,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"The profile ID you are trying to\n" +
				"register is invalid.\n" +
				"Please create a new license.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgProfileIDInUse = WWFCErrorMessage{
		ErrorCode: 22007,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"The friend code you are trying to\n" +
				"register is already in use.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgPayloadInvalid = WWFCErrorMessage{
		ErrorCode: 22008,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"The %[4]s payload is invalid.\n" +
				"Try restarting your game.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}

	WWFCMsgInvalidELO = WWFCErrorMessage{
		ErrorCode: 22009,
		MessageRMC: map[byte]string{
			LangEnglish: "" +
				"You were disconnected from\n" +
				"%[4]s due to an invalid\n" +
				"VR or BR value.\n" +
				"\n" +
				"Error Code: %[1]d",
		},
	}
)

type ErrorEntry struct {
	Code        int    `json:"code"`
	Hex         string `json:"hex"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Fatal       bool   `json:"fatal"`
}

type WWFCMessageEntry struct {
	Code        int    `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
}

func ListGPErrors() []ErrorEntry {
	entries := []struct {
		err      GPError
		name     string
		category string
	}{
		{ErrNone, "ErrNone", "General"},
		{ErrGeneral, "ErrGeneral", "General"},
		{ErrParse, "ErrParse", "General"},
		{ErrNotLoggedIn, "ErrNotLoggedIn", "General"},
		{ErrBadSessionKey, "ErrBadSessionKey", "General"},
		{ErrDatabase, "ErrDatabase", "General"},
		{ErrNetwork, "ErrNetwork", "General"},
		{ErrForcedDisconnect, "ErrForcedDisconnect", "General"},
		{ErrConnectionClosed, "ErrConnectionClosed", "General"},
		{ErrUDPLayer, "ErrUDPLayer", "General"},
		{ErrLogin, "ErrLogin", "Login"},
		{ErrLoginTimeout, "ErrLoginTimeout", "Login"},
		{ErrLoginBadNickname, "ErrLoginBadNickname", "Login"},
		{ErrLoginBadEmail, "ErrLoginBadEmail", "Login"},
		{ErrLoginBadPassword, "ErrLoginBadPassword", "Login"},
		{ErrLoginBadProfile, "ErrLoginBadProfile", "Login"},
		{ErrLoginProfileDeleted, "ErrLoginProfileDeleted", "Login"},
		{ErrLoginConnectionFailed, "ErrLoginConnectionFailed", "Login"},
		{ErrLoginServerAuthFailed, "ErrLoginServerAuthFailed", "Login"},
		{ErrLoginBadUniqueNick, "ErrLoginBadUniqueNick", "Login"},
		{ErrLoginBadPreAuth, "ErrLoginBadPreAuth", "Login"},
		{ErrLoginLoginTicketInvalid, "ErrLoginLoginTicketInvalid", "Login"},
		{ErrLoginLoginTicketExpired, "ErrLoginLoginTicketExpired", "Login"},
		{ErrNewUser, "ErrNewUser", "New User"},
		{ErrNewUserBadNickname, "ErrNewUserBadNickname", "New User"},
		{ErrNewUserBadPassword, "ErrNewUserBadPassword", "New User"},
		{ErrNewUserUniqueNickInvalid, "ErrNewUserUniqueNickInvalid", "New User"},
		{ErrNewUserUniqueNickInUse, "ErrNewUserUniqueNickInUse", "New User"},
		{ErrUpdateUserInfo, "ErrUpdateUserInfo", "Update User Info"},
		{ErrUpdateUserInfoBadEmail, "ErrUpdateUserInfoBadEmail", "Update User Info"},
		{ErrNewProfile, "ErrNewProfile", "New Profile"},
		{ErrNewProfileBadNickname, "ErrNewProfileBadNickname", "New Profile"},
		{ErrNewProfileBadOldNickname, "ErrNewProfileBadOldNickname", "New Profile"},
		{ErrUpdateProfile, "ErrUpdateProfile", "Update Profile"},
		{ErrUpdateProfileBadNickname, "ErrUpdateProfileBadNickname", "Update Profile"},
		{ErrAddFriend, "ErrAddFriend", "Add Friend"},
		{ErrAddFriendBadFrom, "ErrAddFriendBadFrom", "Add Friend"},
		{ErrAddFriendBadNew, "ErrAddFriendBadNew", "Add Friend"},
		{ErrAddFriendAlreadyFriends, "ErrAddFriendAlreadyFriends", "Add Friend"},
		{ErrAddFriendLocalBlock, "ErrAddFriendLocalBlock", "Add Friend"},
		{ErrAddFriendBlocked, "ErrAddFriendBlocked", "Add Friend"},
		{ErrAuthAdd, "ErrAuthAdd", "Auth Add Friend"},
		{ErrAuthAddBadFrom, "ErrAuthAddBadFrom", "Auth Add Friend"},
		{ErrAuthAddBadSignature, "ErrAuthAddBadSignature", "Auth Add Friend"},
		{ErrAuthAddLocalBlock, "ErrAuthAddLocalBlock", "Auth Add Friend"},
		{ErrAuthAddBlocked, "ErrAuthAddBlocked", "Auth Add Friend"},
		{ErrStatus, "ErrStatus", "Status"},
		{ErrMessage, "ErrMessage", "Friend Message"},
		{ErrMessageNotFriends, "ErrMessageNotFriends", "Friend Message"},
		{ErrMessageExtInfoNotSupported, "ErrMessageExtInfoNotSupported", "Friend Message"},
		{ErrMessageFriendOffline, "ErrMessageFriendOffline", "Friend Message"},
		{ErrGetProfile, "ErrGetProfile", "Get Profile"},
		{ErrGetProfileBadProfile, "ErrGetProfileBadProfile", "Get Profile"},
		{ErrDeleteFriend, "ErrDeleteFriend", "Delete Friend"},
		{ErrDeleteFriendNotFriends, "ErrDeleteFriendNotFriends", "Delete Friend"},
		{ErrDeleteProfile, "ErrDeleteProfile", "Delete Profile"},
		{ErrDeleteProfileLastProfile, "ErrDeleteProfileLastProfile", "Delete Profile"},
		{ErrSearch, "ErrSearch", "Profile Search"},
		{ErrSearchConnectFailed, "ErrSearchConnectFailed", "Profile Search"},
		{ErrSearchTimedOut, "ErrSearchTimedOut", "Profile Search"},
		{ErrCheck, "ErrCheck", "User Check"},
		{ErrCheckBadEmail, "ErrCheckBadEmail", "User Check"},
		{ErrCheckBadNickname, "ErrCheckBadNickname", "User Check"},
		{ErrCheckBadPassword, "ErrCheckBadPassword", "User Check"},
		{ErrRevoke, "ErrRevoke", "Revoke Buddy"},
		{ErrRevokeNotFriends, "ErrRevokeNotFriends", "Revoke Buddy"},
		{ErrRegisterUniqueNick, "ErrRegisterUniqueNick", "Register Unique Nick"},
		{ErrRegisterUniqueNickTaken, "ErrRegisterUniqueNickTaken", "Register Unique Nick"},
		{ErrRegisterUniqueNickReserved, "ErrRegisterUniqueNickReserved", "Register Unique Nick"},
		{ErrRegisterUniqueNickBadNamespace, "ErrRegisterUniqueNickBadNamespace", "Register Unique Nick"},
		{ErrRegisterCDKey, "ErrRegisterCDKey", "Register CD Key"},
		{ErrRegisterCDKeyBadKey, "ErrRegisterCDKeyBadKey", "Register CD Key"},
		{ErrRegisterCDKeyAlreadySet, "ErrRegisterCDKeyAlreadySet", "Register CD Key"},
		{ErrRegisterCDKeyAlreadyTaken, "ErrRegisterCDKeyAlreadyTaken", "Register CD Key"},
		{ErrAddBlock, "ErrAddBlock", "Add Block"},
		{ErrAddBlockAlreadyBlocked, "ErrAddBlockAlreadyBlocked", "Add Block"},
		{ErrRemoveBlock, "ErrRemoveBlock", "Remove Block"},
		{ErrRemoveBlockNotBlocked, "ErrRemoveBlockNotBlocked", "Remove Block"},
	}
	out := make([]ErrorEntry, 0, len(entries))
	for _, item := range entries {
		out = append(out, ErrorEntry{
			Code:        item.err.ErrorCode,
			Hex:         fmt.Sprintf("0x%04X", item.err.ErrorCode),
			Name:        item.name,
			Description: item.err.ErrorString,
			Category:    item.category,
			Fatal:       item.err.Fatal,
		})
	}
	return out
}

func ListWWFCMessages() []WWFCMessageEntry {
	entries := []struct {
		msg         WWFCErrorMessage
		title       string
		description string
		action      string
	}{
		{WWFCMsgUnknownLoginError, "Login failed", "The game could not complete login.", "Reconnect once. If it keeps happening, check the server status or ask staff to check the login logs."},
		{WWFCMsgDolphinSetupRequired, "Dolphin setup required", "Dolphin is missing required Wii network setup.", "Use a Dolphin NAND with Wii network setup, install the required certificates, then reconnect."},
		{WWFCMsgProfileBannedTOS, "Account banned", "This game profile is banned from the service.", "Check the ban reason on the website or contact the moderation team."},
		{WWFCMsgProfileRestricted, "Public matchmaking blocked", "This game profile cannot enter public matchmaking.", "Use friends or private rooms if allowed, or contact the moderation team."},
		{WWFCMsgProfileRestrictedNow, "Public matchmaking blocked", "This game profile was just blocked from public matchmaking.", "Use friends or private rooms if allowed, or contact the moderation team."},
		{WWFCMsgProfileRestrictedCustom, "Public matchmaking blocked", "This game profile cannot enter public matchmaking. The in-game message may include a reason.", "Read the in-game reason, then contact the moderation team if you think this is wrong."},
		{WWFCMsgProfileRestrictedNowCustom, "Public matchmaking blocked", "This game profile was just blocked from public matchmaking. The in-game message may include a reason.", "Read the in-game reason, then contact the moderation team if you think this is wrong."},
		{WWFCMsgKickedGeneric, "Disconnected by the service", "The game was disconnected from the service.", "Reconnect. If it happens repeatedly, check the server status or ask staff to check the logs."},
		{WWFCMsgKickedModerator, "Kicked by moderation", "A moderator disconnected this game profile.", "Read the in-game message, wait a moment, then reconnect only if the message allows it."},
		{WWFCMsgKickedRoomHost, "Kicked from private room", "The private room host removed this game profile from the room.", "Ask the room host before joining that private room again."},
		{WWFCMsgKickedRoomHostCustom, "Kicked from private room", "The private room host removed this game profile from the room. The in-game message may include a reason.", "Ask the room host before joining that private room again."},
		{WWFCMsgKickedCustom, "Kicked by moderation", "A moderator disconnected this game profile. The in-game message may include a reason.", "Read the in-game reason before reconnecting."},
		{WWFCMsgConsoleMismatch, "Console mismatch", "This game profile was registered on a different Wii console.", "Use the same Wii or Dolphin NAND that created the profile, or relink the account."},
		{WWFCMsgConsoleMismatchDolphin, "Dolphin NAND mismatch", "Dolphin is using a different NAND than the one used to register this game profile.", "Switch to the correct Dolphin NAND or relink the account with the current NAND."},
		{WWFCMsgProfileIDInvalid, "Invalid profile", "The game profile could not be registered.", "Create a new license or relink the profile, then reconnect."},
		{WWFCMsgProfileIDInUse, "Friend code already used", "This friend code is already linked to another profile.", "Use the profile already linked to that friend code, or ask staff to fix the duplicate."},
		{WWFCMsgPayloadInvalid, "Payload problem", "The game payload is missing, outdated, or invalid.", "Regenerate and reinstall the client payload, then reconnect."},
		{WWFCMsgInvalidELO, "Invalid VR or BR", "The game was disconnected because its VR or BR value is outside the allowed range.", "Restore a valid save value before going online again."},
	}
	out := make([]WWFCMessageEntry, 0, len(entries))
	for _, item := range entries {
		out = append(out, WWFCMessageEntry{
			Code:        item.msg.ErrorCode,
			Title:       item.title,
			Description: item.description,
			Action:      item.action,
		})
	}
	return out
}

func GetDisabledGameMessage(name string) (WWFCErrorMessage, bool) {
	game := common.GetDisabledGameByName(name)
	if game == nil {
		return WWFCErrorMessage{}, false
	}

	return disabledGameMessage(*game), true
}

func disabledGameMessage(game common.DisabledGame) WWFCErrorMessage {
	msg := game.Message
	if msg == "" {
		msg = "This game has been disabled."
	}

	return WWFCErrorMessage{
		ErrorCode: game.ErrorCode,
		MessageRMC: map[byte]string{
			LangEnglish: msg + "\n\nError Code: %[1]d",
		},
	}
}

func (err GPError) GetMessage() string {
	command := common.GameSpyCommand{
		Command:      "error",
		CommandValue: "",
		OtherValues: map[string]string{
			"err":    strconv.Itoa(err.ErrorCode),
			"errmsg": err.ErrorString,
		},
	}

	if err.Fatal {
		command.OtherValues["fatal"] = ""
	}

	return common.CreateGameSpyMessage(command)
}

func getLocalizedWWFCErrorMessage(message WWFCErrorMessage, lang byte) string {
	if errMsg := message.MessageRMC[lang]; errMsg != "" {
		return errMsg
	}

	switch lang {
	case LangSpanishEU:
		if errMsg := message.MessageRMC[LangSpanish]; errMsg != "" {
			return errMsg
		}
	case LangFrenchEU:
		if errMsg := message.MessageRMC[LangFrench]; errMsg != "" {
			return errMsg
		}
	}

	return message.MessageRMC[LangEnglish]
}

func formatWWFCErrorMessage(message string, errorCode int, ngid uint32, reason string) string {
	serviceName := wwfcServiceName()
	errMsg := fmt.Sprintf(message, errorCode, ngid, reason, serviceName)
	if len(utf16.Encode([]rune(errMsg))) <= maxWWFCErrorMessageUTF16Units {
		return errMsg
	}

	reasonRunes := []rune(reason)
	for len(reasonRunes) > 0 {
		reasonRunes = reasonRunes[:len(reasonRunes)-1]
		shortReason := string(reasonRunes) + "..."
		errMsg = fmt.Sprintf(message, errorCode, ngid, shortReason, serviceName)
		if len(utf16.Encode([]rune(errMsg))) <= maxWWFCErrorMessageUTF16Units {
			return errMsg
		}
	}

	return fmt.Sprintf(message, errorCode, ngid, "", serviceName)
}

func wwfcServiceName() (name string) {
	defer func() {
		if recover() != nil {
			name = "WFC Server"
		}
	}()

	name = strings.TrimSpace(common.GetConfig().ServerName)
	if name == "" {
		return "WFC Server"
	}
	return name
}

func (err GPError) GetMessageTranslate(gameName string, region byte, lang byte, cfc uint64, ngid uint32) (string, int) {
	command := common.GameSpyCommand{
		Command:      "error",
		CommandValue: "",
		OtherValues: map[string]string{
			"err":    strconv.Itoa(err.ErrorCode),
			"errmsg": err.ErrorString,
		},
	}

	if err.Fatal {
		command.OtherValues["fatal"] = ""
	}

	if !err.Fatal || err.WWFCMessage.ErrorCode == 0 {
		return common.CreateGameSpyMessage(command), err.WWFCMessage.ErrorCode
	}

	errMsg := ""
	reason := ""
	var wwfcMessage *WWFCErrorMessage
	switch gameName {
	case "mariokartwii":
		reason = err.Reason
		wwfcMessage = &err.WWFCMessage

		if reason == "" {
			reason = "None provided."

			switch engMsg := err.WWFCMessage.MessageRMC[LangEnglish]; engMsg {
			case WWFCMsgKickedCustom.MessageRMC[LangEnglish]:
				wwfcMessage = &WWFCMsgKickedModerator
			case WWFCMsgKickedRoomHostCustom.MessageRMC[LangEnglish]:
				wwfcMessage = &WWFCMsgKickedRoomHost
			case WWFCMsgProfileRestrictedCustom.MessageRMC[LangEnglish]:
				wwfcMessage = &WWFCMsgProfileRestricted
			case WWFCMsgProfileRestrictedNowCustom.MessageRMC[LangEnglish]:
				wwfcMessage = &WWFCMsgProfileRestrictedNow
			}
		}

		errMsg = getLocalizedWWFCErrorMessage(*wwfcMessage, lang)
	}

	if errMsg != "" && wwfcMessage != nil {
		errMsg = formatWWFCErrorMessage(errMsg, wwfcMessage.ErrorCode, ngid, reason)
		errMsgUTF16 := utf16.Encode([]rune(errMsg))
		errMsgByteArray := common.UTF16ToByteArray(errMsgUTF16)
		command.OtherValues["wl:errmsg"] = common.Base64DwcEncoding.EncodeToString(errMsgByteArray)
	}

	command.OtherValues["wl:err"] = strconv.Itoa(err.WWFCMessage.ErrorCode)

	return common.CreateGameSpyMessage(command), err.WWFCMessage.ErrorCode
}

func (g *GameSpySession) replyError(gpErr GPError) {
	logging.Error(g.ModuleName, "Reply error:", gpErr.ErrorString)
	if !g.LoginInfoSet {
		msg := gpErr.GetMessage()
		// logging.Info(g.ModuleName, "Sending error message:", msg)
		err := common.SendPacket(ServerName, g.ConnIndex, []byte(msg))
		if gpErr.Fatal || err != nil {
			if err != nil {
				logging.Error("GPCM", "Failed to send packet:", err)
			}
			common.ShouldNotError(common.CloseConnection(ServerName, g.ConnIndex))
		}
		logging.Event("gpcm_returned_error", map[string]any{
			"profile_id":   g.User.ProfileId,
			"error_code":   gpErr.ErrorCode,
			"error_string": gpErr.ErrorString,
			"fatal":        gpErr.Fatal,
		})
		return
	}

	deviceId := g.User.RestrictedDeviceId
	if deviceId == 0 && len(g.User.NgDeviceId) > 0 {
		deviceId = g.User.NgDeviceId[0]
	}

	msg, wwfcErrorCode := gpErr.GetMessageTranslate(g.GameName, g.Region, g.Language, g.ConsoleFriendCode, deviceId)
	// logging.Info(g.ModuleName, "Sending error message:", msg)
	err := common.SendPacket(ServerName, g.ConnIndex, []byte(msg))
	if gpErr.Fatal || err != nil {
		if err != nil {
			logging.Error("GPCM", "Failed to send packet:", err)
		}
		common.ShouldNotError(common.CloseConnection(ServerName, g.ConnIndex))
	}

	logging.Event("gpcm_returned_error", map[string]any{
		"profile_id":     g.User.ProfileId,
		"error_code":     gpErr.ErrorCode,
		"error_string":   gpErr.ErrorString,
		"fatal":          gpErr.Fatal,
		"dwc_error_code": wwfcErrorCode,
	})
}
