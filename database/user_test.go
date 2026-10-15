package database

import "testing"

func TestIsOpenHostForRequiresEnabledFlag(t *testing.T) {
	user := User{
		OpenHost:     false,
		OpenHostMode: OpenHostModeBlacklist,
	}

	if user.IsOpenHostFor(2) {
		t.Fatal("disabled Open Host should not authorize peers even if mode is blacklist")
	}
}

func TestIsOpenHostForBlacklistAllowsUnlistedPeer(t *testing.T) {
	user := User{
		OpenHost:     true,
		OpenHostMode: OpenHostModeBlacklist,
		OpenHostList: []uint32{3},
	}

	if !user.IsOpenHostFor(2) {
		t.Fatal("blacklist mode should authorize peers not in the list")
	}
	if user.IsOpenHostFor(3) {
		t.Fatal("blacklist mode should not authorize peers in the list")
	}
}
