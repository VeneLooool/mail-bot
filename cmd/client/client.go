package main

import (
	"gitlab.ozon.dev/VeneLooool/homework-2/internal/app/telegram"
)

func main() {
	telegram, err := telegram.NewTelegramBot(60)
	if err != nil {
		panic(err)
	}

	defer telegram.CloseConnectionWithServer()
	go telegram.HandlerUpdatesForUser()

	if err = telegram.StartTelegramBot(); err != nil {
		panic(err)
	}
}
