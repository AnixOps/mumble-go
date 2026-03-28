package state

import "testing"

func TestUserHasMutedField(t *testing.T) {
	u := User{Muted: true, Name: "test"}
	if !u.Muted {
		t.Error("User.Muted should be true")
	}
}
