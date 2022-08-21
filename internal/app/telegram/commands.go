package telegram

import (
	"context"
	"errors"
	botApi "github.com/go-telegram-bot-api/telegram-bot-api"
	api2 "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"google.golang.org/grpc/metadata"
	"strings"
	"time"
)

func (tgBot *TelegramBot) commandStart(update *botApi.Update) error {
	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	_, isFound := tgBot.findUserInInternalDb(update.Message.From.ID)
	if isFound {
		msg := botApi.NewMessage(update.Message.Chat.ID, "Choose one option")
		msg.ReplyMarkup = MainMenuButton
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
		return nil
	}

	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx,
		"sender", "Client",
		"when", time.Now().Format(time.RFC3339),
		"sender", "route256",
	)

	userSearchResp, err := tgBot.client.CustomerSearch(ctx, &api2.CustomerSearchReq{Id: 1, TelegramID: int64(update.Message.From.ID)})
	if err != nil {
		return err
	}

	if userSearchResp.GetStatus() == api2.Status_USERNOTFOUND {
		createUserResp, err := tgBot.client.CreateUser(ctx, &api2.CreateUserReq{Id: 1, TelegramID: int64(update.Message.From.ID)})
		if err != nil {
			return err
		}

		if createUserResp.GetStatus() != api2.Status_SUCCESS {
			return errors.New("unimplemented error")
		}
	}

	newUser := &TelegramUser{
		telegramId: update.Message.From.ID,
		chatId:     int(update.Message.Chat.ID),
		mailServ:   make([]*MailService, 0),
	}
	tgBot.users = append(tgBot.users, newUser)

	msg := botApi.NewMessage(update.Message.Chat.ID, "Choose one option")
	msg.ReplyMarkup = MainMenuButton
	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}

	return nil
}

func (tgBot *TelegramBot) commandAddService(update *botApi.Update) error {

	_, isFound := tgBot.findUserInInternalDb(update.Message.From.ID)
	if !isFound {
		if err := tgBot.commandStart(update); err != nil {
			return err
		}
		return nil
	}

	msg := botApi.NewMessage(update.Message.Chat.ID, "Choose mail service")
	serviceNames := botApi.NewInlineKeyboardRow()
	for _, internalName := range tgBot.services {
		externalName := internalName[strings.IndexRune(internalName, '.')+1 : strings.IndexRune(internalName, ':')]
		serviceNames = append(serviceNames, botApi.NewInlineKeyboardButtonData(externalName, internalName+AddMailServiceDescription))
	}
	msg.ReplyMarkup = botApi.NewInlineKeyboardMarkup(serviceNames)

	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (tgBot *TelegramBot) commandGoToMailService(update *botApi.Update) error {
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

	if len(user.mailServ) == 0 {
		msg := botApi.NewMessage(update.Message.Chat.ID, "You don't have any mail services :(")
		msg.ReplyMarkup = nil
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	} else {
		msg := botApi.NewMessage(update.Message.Chat.ID, "Choose mail service")
		serviceNames := botApi.NewInlineKeyboardRow()
		for _, service := range user.mailServ {
			serviceNames = append(serviceNames,
				botApi.NewInlineKeyboardButtonData(service.userName, service.userName+"#"+service.internalName+SettingMailServiceDescription),
			)
		}
		msg.ReplyMarkup = botApi.NewInlineKeyboardMarkup(serviceNames)
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

func (tgBot *TelegramBot) commandSyncServices(update *botApi.Update) error {
	_, isFound := tgBot.findUserInInternalDb(update.Message.From.ID)
	if !isFound {
		if err := tgBot.commandStart(update); err != nil {
			return err
		}
	}
	user, _ := tgBot.findUserInInternalDb(update.Message.From.ID)

	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	user.userMutex.Lock()
	defer user.userMutex.Unlock()

	resp, err := tgBot.client.GetListAvailableMailServices(tgBot.ctx, &api2.GetListAvailableMailServicesReq{
		Id:         1,
		TelegramID: int64(user.telegramId),
	})
	if err != nil {
		return err
	}

	if resp.GetStatus() == api2.Status_FAIL {
		msg := botApi.NewMessage(update.Message.Chat.ID, "You don't have any mail services :(")
		msg.ReplyMarkup = nil
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	} else if resp.GetStatus() == api2.Status_SUCCESS {
		for _, service := range resp.GetAvailableMailServices() {
			isServiceExistInInternalDB := false

			for _, internalServ := range user.mailServ {
				if internalServ.userName == service.GetUserName() && internalServ.internalName == service.GetMailServiceNameInternalRep() {
					isServiceExistInInternalDB = true
					break
				}
			}

			if !isServiceExistInInternalDB {
				user.mailServ = append(user.mailServ, &MailService{
					internalName: service.GetMailServiceNameInternalRep(),
					externalName: service.MailServiceNameExternalRep,
					userName:     service.GetUserName(),
					isLogin:      true,
				})
			}
		}
		msg := botApi.NewMessage(update.Message.Chat.ID, "All available services were synchronized")
		msg.ReplyMarkup = nil
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	}
	return nil
}
