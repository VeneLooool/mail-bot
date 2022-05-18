package telegram

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

var MainMenuButton = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Add mail service"),
		tgbotapi.NewKeyboardButton("Go to mail service"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Synchronize your mail service with main serv"),
	),
)
var (
	AddMailServiceDescription     = "_ADDMAIL"
	SettingMailServiceDescription = "_SETTINGS"
	TurnOnConstUpdateSettings     = "_TurnOnContUpdateSettings"
	TurnOffConstUpdateSettings    = "_TurnOffContUpdateSettings"
	GetLastMessageSettings        = "_GetLastMessageSettings"
)
