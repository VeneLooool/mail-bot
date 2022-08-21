package telegram

import (
	"context"
	"errors"
	"fmt"
	botApi "github.com/go-telegram-bot-api/telegram-bot-api"
	api2 "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"google.golang.org/grpc/metadata"
	"log"
	"strings"
	"time"
)

func (tgBot *TelegramBot) HandlerUpdatesForUser() {
	for {
		for _, user := range tgBot.users {
			user.userMutex.Lock()
			if user.hasOnlineUpdate {
				tgBot.clientMutex.Lock()

				resp, err := tgBot.client.CheckForUpdates(tgBot.ctx, &api2.CheckForUpdatesReq{
					Id:         1,
					TelegramID: int64(user.telegramId),
				})

				if err != nil {
					tgBot.clientMutex.Unlock()
					user.userMutex.Unlock()
					log.Println("error happened in updates")
					return
				}

				if resp.GetStatus() == api2.Status_SUCCESS {

					for _, updateMessage := range resp.GetMessages() {
						internalName := updateMessage.GetMailServiceName()
						externalName := internalName[strings.IndexRune(internalName, '.')+1 : strings.IndexRune(internalName, ':')]

						tgBot.botMutex.Lock()
						msg := botApi.NewMessage(int64(user.chatId), externalName+"\n"+updateMessage.GetUserName()+"\n"+updateMessage.GetMessageHeader()+"\n"+updateMessage.GetMessageBody())

						if _, err := tgBot.bot.Send(msg); err != nil {
							tgBot.botMutex.Unlock()
							tgBot.clientMutex.Unlock()
							user.userMutex.Unlock()
							log.Println(err)
							return
						}

						tgBot.botMutex.Unlock()
					}
				}
				tgBot.clientMutex.Unlock()
			}
			user.userMutex.Unlock()
		}
		time.Sleep(50 * time.Second)
	}
}

func (tgBot *TelegramBot) handlerAddMailService(update *botApi.Update) error {

	user, isFound := tgBot.findUserInInternalDb(update.CallbackQuery.From.ID)
	if !isFound {
		return errors.New("unimplemented error")
	}

	user.userMutex.Lock()
	defer user.userMutex.Unlock()
	if user.inputFlags.username || user.inputFlags.password {
		user.inputFlags.username = false
		user.inputFlags.password = false

		if len(user.mailServ) != 0 {
			user.mailServ[len(user.mailServ)-1] = nil
			user.mailServ = user.mailServ[:len(user.mailServ)-1]
		} else {
			return errors.New("unimplemented error")
		}
	}

	user.inputFlags.username = true
	user.mailServ = append(user.mailServ, &MailService{
		internalName: update.CallbackQuery.Data[:strings.IndexRune(update.CallbackQuery.Data, '_')],
		externalName: update.CallbackQuery.Data[strings.IndexRune(update.CallbackQuery.Data, '.')+1 : strings.IndexRune(update.CallbackQuery.Data, ':')],
	})
	msg := botApi.NewMessage(int64(user.chatId), "pls enter username:")
	msg.ReplyMarkup = nil

	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (tgBot *TelegramBot) handlerSetupMailService(update *botApi.Update) error {
	user, isFound := tgBot.findUserInInternalDb(update.CallbackQuery.From.ID)
	if !isFound {
		return errors.New("unimplemented error")
	}
	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	indexOfKeyWord := strings.Index(update.CallbackQuery.Data, SettingMailServiceDescription)
	indexOfKeyValue := strings.Index(update.CallbackQuery.Data, "#")

	deepSettingsForMailServiceKeyboard := botApi.NewInlineKeyboardMarkup(
		botApi.NewInlineKeyboardRow(
			botApi.NewInlineKeyboardButtonData("Get last Message",
				update.CallbackQuery.Data[:indexOfKeyValue]+"#"+update.CallbackQuery.Data[indexOfKeyValue+1:indexOfKeyWord]+GetLastMessageSettings),
		),

		botApi.NewInlineKeyboardRow(
			botApi.NewInlineKeyboardButtonData("Turn on",
				update.CallbackQuery.Data[:indexOfKeyValue]+"#"+update.CallbackQuery.Data[indexOfKeyValue+1:indexOfKeyWord]+TurnOnConstUpdateSettings),

			botApi.NewInlineKeyboardButtonData("Turn off",
				update.CallbackQuery.Data[:indexOfKeyValue]+"#"+update.CallbackQuery.Data[indexOfKeyValue+1:indexOfKeyWord]+TurnOffConstUpdateSettings),
		),
	)

	msg := botApi.NewEditMessageReplyMarkup(int64(user.chatId), update.CallbackQuery.Message.MessageID, deepSettingsForMailServiceKeyboard)
	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}

	return nil
}

func (tgBot *TelegramBot) handlerDefaultMessage(update *botApi.Update) error {

	user, isFound := tgBot.findUserInInternalDb(update.Message.From.ID)
	if !isFound {
		if err := tgBot.commandStart(update); err != nil {
			return err
		}
		return nil
	}

	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	user.userMutex.Lock()
	defer user.userMutex.Unlock()

	if user.inputFlags.username {
		user.mailServ[len(user.mailServ)-1].userName = update.Message.Text
		user.inputFlags.username = false
		user.inputFlags.password = true

		msg := botApi.NewMessage(int64(user.chatId), "pls enter password:")
		msg.ReplyMarkup = nil
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	} else if user.inputFlags.password {
		user.mailServ[len(user.mailServ)-1].password = update.Message.Text
		user.inputFlags.password = false

		ctx := context.Background()
		ctx = metadata.AppendToOutgoingContext(ctx,
			"sender", "Client",
			"when", time.Now().Format(time.RFC3339),
			"sender", "route256",
		)

		resp, err := tgBot.client.AddNewMailService(ctx, &api2.AddNewMailServiceReq{
			Id:              1,
			MailServiceName: user.mailServ[len(user.mailServ)-1].internalName,
			Login:           user.mailServ[len(user.mailServ)-1].userName,
			Password:        user.mailServ[len(user.mailServ)-1].password,
			TelegramID:      int64(user.telegramId),
		})
		if err != nil {
			return err
		}

		if resp.GetStatus() == api2.Status_INCORRECTLOGINDATA {
			user.inputFlags.username = true
			msg := botApi.NewMessage(int64(user.chatId), "Incorrect login data\ninput username again")
			msg.ReplyMarkup = nil
			if _, err := tgBot.bot.Send(msg); err != nil {
				return err
			}
			return nil
		}
		user.mailServ[len(user.mailServ)-1].isLogin = true
		msg := botApi.NewMessage(int64(user.chatId), "mail address successfully added")
		msg.ReplyMarkup = nil
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	} else {
		msg := botApi.NewMessage(int64(user.chatId), "sorry i don't know this command\ni'm just a bot :)")
		msg.ReplyMarkup = nil
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

func (tgBot *TelegramBot) handlerTurnOnConstUpdate(update *botApi.Update) error {
	user, isFound := tgBot.findUserInInternalDb(update.CallbackQuery.From.ID)
	if !isFound {
		return errors.New("unimplemented error")
	}

	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	user.userMutex.Lock()
	defer user.userMutex.Unlock()

	indexOfKeyWord := strings.Index(update.CallbackQuery.Data, TurnOnConstUpdateSettings)
	indexOfKeyValue := strings.Index(update.CallbackQuery.Data, "#")
	internalName := update.CallbackQuery.Data[indexOfKeyValue+1 : indexOfKeyWord]
	userName := update.CallbackQuery.Data[:indexOfKeyValue]

	mailService, isFound := user.findMailServiceInInternalDb(userName, internalName)
	if !isFound {
		return errors.New("unimplemented error")
	}

	resp, err := tgBot.client.ConstantlyUpdate(tgBot.ctx, &api2.ConstantlyUpdateReq{
		Id:              1,
		MailServiceName: internalName,
		Switch:          true,
		Login:           userName,
		TelegramID:      int64(user.telegramId),
	})
	if err != nil {
		return err
	}

	if resp.GetStatus() == api2.Status_USERNOTFOUND || resp.GetStatus() == api2.Status_MAILSERVICENOTFOUND {
		msg := botApi.NewMessage(int64(user.chatId), "Something went wrong")
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
		return nil
	}
	user.hasOnlineUpdate = true
	mailService.onlineUpdate = true

	msg := botApi.NewMessage(int64(user.chatId), "Successfully")
	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (tgBot *TelegramBot) handlerTurnOffConstUpdate(update *botApi.Update) error {
	user, isFound := tgBot.findUserInInternalDb(update.CallbackQuery.From.ID)
	if !isFound {
		return errors.New("unimplemented error")
	}

	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	user.userMutex.Lock()
	defer user.userMutex.Unlock()

	indexOfKeyWord := strings.Index(update.CallbackQuery.Data, TurnOffConstUpdateSettings)
	indexOfKeyValue := strings.Index(update.CallbackQuery.Data, "#")
	internalName := update.CallbackQuery.Data[indexOfKeyValue+1 : indexOfKeyWord]
	userName := update.CallbackQuery.Data[:indexOfKeyValue]

	mailService, isFound := user.findMailServiceInInternalDb(userName, internalName)
	if !isFound {
		return errors.New("unimplemented error")
	}

	resp, err := tgBot.client.ConstantlyUpdate(tgBot.ctx, &api2.ConstantlyUpdateReq{
		Id:              1,
		MailServiceName: internalName,
		Switch:          false,
		Login:           userName,
		TelegramID:      int64(user.telegramId),
	})
	if err != nil {
		return err
	}

	if resp.GetStatus() == api2.Status_USERNOTFOUND || resp.GetStatus() == api2.Status_MAILSERVICENOTFOUND {
		msg := botApi.NewMessage(int64(user.chatId), "Something went wrong")
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
		return nil
	}

	mailService.onlineUpdate = false
	isAnyServicesRemained := false
	for _, service := range user.mailServ {
		if service.onlineUpdate {
			isAnyServicesRemained = true
		}
	}
	user.hasOnlineUpdate = isAnyServicesRemained

	msg := botApi.NewMessage(int64(user.chatId), "Successfully")
	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (tgBot *TelegramBot) handlerGetLastMessage(update *botApi.Update) error {
	user, isFound := tgBot.findUserInInternalDb(update.CallbackQuery.From.ID)
	if !isFound {
		return errors.New("unimplemented error")
	}

	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	user.userMutex.Lock()
	defer user.userMutex.Unlock()

	indexOfKeyWord := strings.Index(update.CallbackQuery.Data, GetLastMessageSettings)
	indexOfKeyValue := strings.Index(update.CallbackQuery.Data, "#")
	internalName := update.CallbackQuery.Data[indexOfKeyValue+1 : indexOfKeyWord]
	userName := update.CallbackQuery.Data[:indexOfKeyValue]

	_, isFound = user.findMailServiceInInternalDb(userName, internalName)
	if !isFound {
		return errors.New("unimplemented error")
	}

	resp, err := tgBot.client.GetLastMessages(tgBot.ctx, &api2.GetLastMessageReq{
		Id:              1,
		AmountMessages:  1,
		MailServiceName: internalName,
		UserName:        userName,
		TelegramID:      int64(user.telegramId),
	})
	if err != nil {
		return err
	}
	fmt.Println(resp.GetStatus())
	if resp.GetStatus() == api2.Status_SUCCESS {
		lastMessages := resp.GetMessages()
		for _, message := range lastMessages {
			msg := botApi.NewMessage(int64(user.chatId), message.GetUserName()+"\n"+message.GetMessageHeader()+"\n"+message.GetMessageBody())
			if _, err := tgBot.bot.Send(msg); err != nil {
				return err
			}
		}
	}
	return nil
}
