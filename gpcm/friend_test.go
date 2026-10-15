package gpcm

import (
	"dwc-server-advanced/database"
	"testing"
)

func TestOpenHostDisabledRemovesTemporaryFriend(t *testing.T) {
	session := &GameSpySession{
		User: database.User{
			ProfileId:    1,
			OpenHostMode: database.OpenHostModeDisabled,
		},
		FriendList:         []uint32{2},
		AuthFriendList:     []uint32{2},
		OpenHostFriendList: []uint32{2},
	}

	session.openHostDisabledLocked(false)

	if session.isFriendAdded(2) {
		t.Fatal("temporary Open Host friend was not removed from FriendList")
	}
	if session.isFriendAuthorized(2) {
		t.Fatal("temporary Open Host friend was not removed from AuthFriendList")
	}
	if session.isOpenHostFriend(2) {
		t.Fatal("temporary Open Host friend marker was not removed")
	}
}

func TestOpenHostDisabledKeepsManualFriend(t *testing.T) {
	session := &GameSpySession{
		User: database.User{
			ProfileId:    1,
			OpenHostMode: database.OpenHostModeDisabled,
		},
		FriendList:     []uint32{2},
		AuthFriendList: []uint32{2},
	}

	session.openHostDisabledLocked(false)

	if !session.isFriendAdded(2) {
		t.Fatal("manual friend was removed from FriendList")
	}
	if !session.isFriendAuthorized(2) {
		t.Fatal("manual friend was removed from AuthFriendList")
	}
}

func TestOpenHostFriendCanBePromotedToManualFriend(t *testing.T) {
	session := &GameSpySession{}

	session.addOpenHostFriend(2)
	session.removeOpenHostFriendMarker(2)

	if !session.isFriendAdded(2) {
		t.Fatal("promoted friend should remain in FriendList")
	}
	if session.isOpenHostFriend(2) {
		t.Fatal("promoted friend should not keep Open Host marker")
	}
}
