package telegram

import "testing"

func TestFindUser(t *testing.T) {
	testBot, err := NewTelegramBot(60)
	if err != nil {
		t.Error(err)
	}
	testUser, isFound := testBot.findUserInInternalDb(0)
	if isFound || testUser != nil {
		t.Errorf("no user create already")
	}
}
func TestFindService(t *testing.T) {
	testBot, err := NewTelegramBot(60)
	if err != nil {
		t.Error(err)
	}
	testUser, _ := testBot.findUserInInternalDb(0)
	testService, isFound := testUser.findMailServiceInInternalDb("", "")
	if testService != nil || isFound {
		t.Errorf("critical error, find serrvice on nil")
	}

}
