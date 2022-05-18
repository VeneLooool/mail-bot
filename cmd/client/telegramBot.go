package main

import (
	"gitlab.ozon.dev/VeneLooool/homework-2/internal/app/telegram"
)

func main() {
	tgBot, err := telegram.CreateTelegramBot("localhost:8080", "5323621344:AAFiygB5y1qHek82eLEO9pi-iRzNJHB1-aQ", 60)
	if err != nil {
		panic(err)
	}

	defer tgBot.CloseConnectionWithServer()
	go tgBot.HandlerUpdatesForUser()
	err = tgBot.StartTelegramBot()
	if err != nil {
		panic(err)
	}
}
