package telegram

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

var MainMenuButton = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Add mail service"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Go to mail service"),
	),
)
