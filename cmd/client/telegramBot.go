package main

import (
	"gitlab.ozon.dev/VeneLooool/homework-2/internal/app/telegram"
)

func main() {
	//bot, err := tgbotapi.NewBotAPI("5323621344:AAFiygB5y1qHek82eLEO9pi-iRzNJHB1-aQ")
	//if err != nil {
	//	log.Panic(err)
	//}
	//BotUpdate := tgbotapi.NewUpdate(0)
	//BotUpdate.Timeout = 60
	//updates, err := bot.GetUpdatesChan(BotUpdate)
	//for update := range updates {
	//	if update.Message != nil { // If we got a message
	//		//log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
	//		//log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
	//		//update.Message.From.ID
	//		//msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
	//		//msg.ReplyToMessageID = update.Message.MessageID
	//		//
	//		//bot.Send(msg)
	//	}
	//}

	tgBot, err := telegram.CreateTelegramBot("localhost:8080", "5323621344:AAFiygB5y1qHek82eLEO9pi-iRzNJHB1-aQ", 60)
	if err != nil {
		panic(err)
	}

	defer tgBot.CloseConnectionWithServer()
	err = tgBot.StartTelegramBot()
	if err != nil {
		panic(err)
	}
}
