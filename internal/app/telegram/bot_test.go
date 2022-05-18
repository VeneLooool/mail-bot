package telegram

import "testing"

func TestFindUser(t *testing.T) {
	testBot, err := CreateTelegramBot("localhost:8080", "5323621344:AAFiygB5y1qHek82eLEO9pi-iRzNJHB1-aQ", 60)
	if err != nil {
		t.Error(err)
	}
	testUser, isFound := testBot.findUserInInternalDb(0)
	if isFound || testUser != nil {
		t.Errorf("no user create already")
	}
}
func TestFindMailService(t *testing.T) {
	testBot, err := CreateTelegramBot("localhost:8080", "5323621344:AAFiygB5y1qHek82eLEO9pi-iRzNJHB1-aQ", 60)
	if err != nil {
		t.Error(err)
	}
	testUser, _ := testBot.findUserInInternalDb(0)
	testMailService, isFound := testUser.findMailServiceInInternalDb("", "")
	if testMailService != nil || isFound {
		t.Errorf("critical error, find serrvice on nil")
	}

}
