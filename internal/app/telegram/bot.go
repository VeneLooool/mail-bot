package telegram

import (
	"context"
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	homework_2 "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"gitlab.ozon.dev/VeneLooool/homework-2/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"log"
	"strings"
	"sync"
	"time"
)

type InputFlag struct {
	username bool
	password bool
}

type MailService struct {
	internalName     string
	externalName     string
	userName         string
	password         string
	constantlyUpdate bool
	isLogin          bool
}

type TelegramUser struct {
	telegramId           int
	chatId               int
	mailServ             []*MailService
	updateMutex          sync.Mutex      //TODO возможно стоит убрать?
	update               tgbotapi.Update //TODO для будущего переработки(обработка каждого юзера на отдельных потоках)
	inputFlags           InputFlag
	userMutex            sync.Mutex
	oneOrMoreConstUpdate bool
}

type TelegramBot struct {
	token                 string
	users                 []*TelegramUser
	botMutex              sync.Mutex
	bot                   *tgbotapi.BotAPI
	updatesConfig         tgbotapi.UpdateConfig
	updates               tgbotapi.UpdatesChannel
	client                homework_2.MailServClient
	clientMutex           sync.Mutex
	connectionToServer    *grpc.ClientConn
	ctx                   context.Context
	availableMailServices []string
}

func CreateTelegramBot(timeout int) (telegramBot TelegramBot, err error) {
	configuration, err := config.GetConfig()
	if err != nil {
		return TelegramBot{}, err
	}

	telegramBot.availableMailServices = configuration.GetAvailableMailServices() //[]string{"imap.gmail.com:993", "imap.yandex.ru:993"}
	telegramBot.token = configuration.GetTelegramToken()

	telegramBot.users = make([]*TelegramUser, 0)
	telegramBot.ctx = context.Background()

	telegramBot.ctx = metadata.AppendToOutgoingContext(telegramBot.ctx,
		"sender", "testClient",
		"when", time.Now().Format(time.RFC3339),
		"sender", "route256",
	)

	telegramBot.bot, err = tgbotapi.NewBotAPI(telegramBot.token)
	if err != nil {
		return TelegramBot{}, err
	}

	telegramBot.updatesConfig = tgbotapi.NewUpdate(0)
	telegramBot.updatesConfig.Timeout = timeout

	telegramBot.updates, err = telegramBot.bot.GetUpdatesChan(telegramBot.updatesConfig)
	if err != nil {
		return TelegramBot{}, err
	}

	telegramBot.connectionToServer, err = grpc.Dial("localhost"+configuration.GetServerAddressAndPort(), grpc.WithInsecure())
	if err != nil {
		return TelegramBot{}, nil
	}

	telegramBot.client = homework_2.NewMailServClient(telegramBot.connectionToServer)
	return telegramBot, nil
}

func (tgBot *TelegramBot) StartTelegramBot() error {
	for update := range tgBot.updates {
		if update.Message != nil {
			fmt.Println(update.Message.Text)
			switch update.Message.Text {
			case "/start":
				if err := tgBot.commandStart(&update); err != nil {
					return err
				}
			case "Add mail service":
				if err := tgBot.commandAddMailService(&update); err != nil {
					return err
				}
			case "Go to mail service":
				if err := tgBot.commandGoToMailService(&update); err != nil {
					return err
				}
			case "Synchronize your mail service with main serv":
				if err := tgBot.commandSynchronizeMailServiceWithMainServer(&update); err != nil {
					return err
				}
			default:
				{
					if err := tgBot.handlerDefaultMessage(&update); err != nil {
						return err
					}
				}
			}
		}
		if update.CallbackQuery != nil {
			fmt.Println(update.CallbackQuery.Data)
			if strings.Index(update.CallbackQuery.Data, AddMailServiceDescription) != -1 {
				if err := tgBot.handlerAddMailService(&update); err != nil {
					return err
				}
			}
			if strings.Index(update.CallbackQuery.Data, SettingMailServiceDescription) != -1 {
				if err := tgBot.handlerSetupMailService(&update); err != nil {
					return err
				}
			}
			if strings.Index(update.CallbackQuery.Data, TurnOnConstUpdateSettings) != -1 {
				if err := tgBot.handlerTurnOnConstUpdate(&update); err != nil {
					return err
				}
			}
			if strings.Index(update.CallbackQuery.Data, TurnOffConstUpdateSettings) != -1 {
				if err := tgBot.handlerTurnOffConstUpdate(&update); err != nil {
					return err
				}
			}
			if strings.Index(update.CallbackQuery.Data, GetLastMessageSettings) != -1 {
				if err := tgBot.handlerGetLastMessage(&update); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (tgBot *TelegramBot) HandlerUpdatesForUser() {
	for {
		for _, user := range tgBot.users {
			user.userMutex.Lock()
			if user.oneOrMoreConstUpdate {
				tgBot.clientMutex.Lock()

				checkForUpdatesResp, err := tgBot.client.CheckForUpdates(tgBot.ctx, &homework_2.CheckForUpdatesReq{
					Id:         1,
					TelegramID: int64(user.telegramId),
				})

				if err != nil {
					tgBot.clientMutex.Unlock()
					user.userMutex.Unlock()
					log.Println("error happened in updates")
					return
				}

				if checkForUpdatesResp.GetStatus() == homework_2.Status_SUCCESS {

					for _, updateMessage := range checkForUpdatesResp.GetMessages() {
						internalName := updateMessage.GetMailServiceName()
						externalName := internalName[strings.IndexRune(internalName, '.')+1 : strings.IndexRune(internalName, ':')]

						tgBot.botMutex.Lock()
						msg := tgbotapi.NewMessage(int64(user.chatId), externalName+"\n"+updateMessage.GetUserName()+"\n"+updateMessage.GetMessageHeader()+"\n"+updateMessage.GetMessageBody())

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

func (tgBot *TelegramBot) commandStart(update *tgbotapi.Update) error {
	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	_, isFound := tgBot.findUserInInternalDb(update.Message.From.ID)
	if isFound {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Choose one option")
		msg.ReplyMarkup = MainMenuButton
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
		return nil
	}

	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx,
		"sender", "testClient",
		"when", time.Now().Format(time.RFC3339),
		"sender", "route256",
	)

	userSearchResp, err := tgBot.client.CustomerSearch(ctx, &homework_2.CustomerSearchReq{Id: 1, TelegramID: int64(update.Message.From.ID)})
	if err != nil {
		return err
	}

	if userSearchResp.GetStatus() == homework_2.Status_USERNOTFOUND {
		createUserResp, err := tgBot.client.CreateUser(ctx, &homework_2.CreateUserReq{Id: 1, TelegramID: int64(update.Message.From.ID)})
		if err != nil {
			return err
		}

		if createUserResp.GetStatus() != homework_2.Status_SUCCESS {
			return errors.New("unimplemented error")
		}
	}

	newUser := &TelegramUser{
		telegramId: update.Message.From.ID,
		chatId:     int(update.Message.Chat.ID),
		mailServ:   make([]*MailService, 0),
	}
	tgBot.users = append(tgBot.users, newUser)

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Choose one option")
	msg.ReplyMarkup = MainMenuButton
	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}

	return nil
}

func (tgBot *TelegramBot) commandAddMailService(update *tgbotapi.Update) error {

	_, isFound := tgBot.findUserInInternalDb(update.Message.From.ID)
	if !isFound {
		if err := tgBot.commandStart(update); err != nil {
			return err
		}
		return nil
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Choose mail service")
	mailServNameInlineRow := tgbotapi.NewInlineKeyboardRow()
	for _, internalName := range tgBot.availableMailServices {
		externalName := internalName[strings.IndexRune(internalName, '.')+1 : strings.IndexRune(internalName, ':')]
		mailServNameInlineRow = append(mailServNameInlineRow, tgbotapi.NewInlineKeyboardButtonData(externalName, internalName+AddMailServiceDescription))
	}
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(mailServNameInlineRow)

	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (tgBot *TelegramBot) commandGoToMailService(update *tgbotapi.Update) error {
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
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You don't have any mail services :(")
		msg.ReplyMarkup = nil
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	} else {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Choose mail service")
		availableMailServInlineRow := tgbotapi.NewInlineKeyboardRow()
		for _, service := range user.mailServ {
			availableMailServInlineRow = append(availableMailServInlineRow,
				tgbotapi.NewInlineKeyboardButtonData(service.userName, service.userName+"#"+service.internalName+SettingMailServiceDescription))
		}
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(availableMailServInlineRow)

		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

func (tgBot *TelegramBot) commandSynchronizeMailServiceWithMainServer(update *tgbotapi.Update) error {
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

	getListAvailableMailServResp, err := tgBot.client.GetListAvailableMailServices(tgBot.ctx, &homework_2.GetListAvailableMailServicesReq{
		Id:         1,
		TelegramID: int64(user.telegramId),
	})
	if err != nil {
		return err
	}

	if getListAvailableMailServResp.GetStatus() == homework_2.Status_FAIL {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You don't have any mail services :(")
		msg.ReplyMarkup = nil
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	} else if getListAvailableMailServResp.GetStatus() == homework_2.Status_SUCCESS {
		for _, service := range getListAvailableMailServResp.GetAvailableMailServices() {
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
					isLogin:      true, //TODO поправить и отправлять залогинен ли он(сейчас по факту нет дб, следовательно все пользователи которые вошли залогинены)
				})
			}
		}
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "All available services were synchronized")
		msg.ReplyMarkup = nil
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

func (tgBot *TelegramBot) handlerAddMailService(update *tgbotapi.Update) error {

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
	msg := tgbotapi.NewMessage(int64(user.chatId), "pls enter username:")
	msg.ReplyMarkup = nil

	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (tgBot *TelegramBot) handlerSetupMailService(update *tgbotapi.Update) error {
	user, isFound := tgBot.findUserInInternalDb(update.CallbackQuery.From.ID)
	if !isFound {
		return errors.New("unimplemented error")
	}
	tgBot.botMutex.Lock()
	defer tgBot.botMutex.Unlock()

	indexOfKeyWord := strings.Index(update.CallbackQuery.Data, SettingMailServiceDescription)
	indexOfKeyValue := strings.Index(update.CallbackQuery.Data, "#")

	deepSettingsForMailServiceKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Get last Message",
				update.CallbackQuery.Data[:indexOfKeyValue]+"#"+update.CallbackQuery.Data[indexOfKeyValue+1:indexOfKeyWord]+GetLastMessageSettings),
		),

		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Turn on",
				update.CallbackQuery.Data[:indexOfKeyValue]+"#"+update.CallbackQuery.Data[indexOfKeyValue+1:indexOfKeyWord]+TurnOnConstUpdateSettings),

			tgbotapi.NewInlineKeyboardButtonData("Turn off",
				update.CallbackQuery.Data[:indexOfKeyValue]+"#"+update.CallbackQuery.Data[indexOfKeyValue+1:indexOfKeyWord]+TurnOffConstUpdateSettings),
		),
	)

	msg := tgbotapi.NewEditMessageReplyMarkup(int64(user.chatId), update.CallbackQuery.Message.MessageID, deepSettingsForMailServiceKeyboard)
	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}

	return nil
}

func (tgBot *TelegramBot) handlerDefaultMessage(update *tgbotapi.Update) error {

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

		msg := tgbotapi.NewMessage(int64(user.chatId), "pls enter password:")
		msg.ReplyMarkup = nil
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	} else if user.inputFlags.password {
		user.mailServ[len(user.mailServ)-1].password = update.Message.Text
		user.inputFlags.password = false

		ctx := context.Background()
		ctx = metadata.AppendToOutgoingContext(ctx,
			"sender", "testClient",
			"when", time.Now().Format(time.RFC3339),
			"sender", "route256",
		)

		addNewMailServResp, err := tgBot.client.AddNewMailService(ctx, &homework_2.AddNewMailServiceReq{
			Id:              1,
			MailServiceName: user.mailServ[len(user.mailServ)-1].internalName,
			Login:           user.mailServ[len(user.mailServ)-1].userName,
			Password:        user.mailServ[len(user.mailServ)-1].password,
			TelegramID:      int64(user.telegramId),
		})
		if err != nil {
			return err
		}

		if addNewMailServResp.GetStatus() == homework_2.Status_INCORRECTLOGINDATA {
			user.inputFlags.username = true
			msg := tgbotapi.NewMessage(int64(user.chatId), "Incorrect login data\ninput username again")
			msg.ReplyMarkup = nil
			if _, err := tgBot.bot.Send(msg); err != nil {
				return err
			}
			return nil
		}
		user.mailServ[len(user.mailServ)-1].isLogin = true
		msg := tgbotapi.NewMessage(int64(user.chatId), "mail address successfully added")
		msg.ReplyMarkup = nil
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	} else {
		msg := tgbotapi.NewMessage(int64(user.chatId), "sorry i don't know this command\ni'm just a bot :)")
		msg.ReplyMarkup = nil
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
	}

	return nil
}

func (tgBot *TelegramBot) handlerTurnOnConstUpdate(update *tgbotapi.Update) error {
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

	turnOnConstUpdateResp, err := tgBot.client.ConstantlyUpdate(tgBot.ctx, &homework_2.ConstantlyUpdateReq{
		Id:              1,
		MailServiceName: internalName,
		Switch:          true,
		Login:           userName,
		TelegramID:      int64(user.telegramId),
	})
	if err != nil {
		return err
	}

	if turnOnConstUpdateResp.GetStatus() == homework_2.Status_USERNOTFOUND || turnOnConstUpdateResp.GetStatus() == homework_2.Status_MAILSERVICENOTFOUND {
		msg := tgbotapi.NewMessage(int64(user.chatId), "Something went wrong")
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
		return nil
	}
	user.oneOrMoreConstUpdate = true
	mailService.constantlyUpdate = true

	msg := tgbotapi.NewMessage(int64(user.chatId), "Successfully")
	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (tgBot *TelegramBot) handlerTurnOffConstUpdate(update *tgbotapi.Update) error {
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

	turnOffConstUpdateResp, err := tgBot.client.ConstantlyUpdate(tgBot.ctx, &homework_2.ConstantlyUpdateReq{
		Id:              1,
		MailServiceName: internalName,
		Switch:          false,
		Login:           userName,
		TelegramID:      int64(user.telegramId),
	})
	if err != nil {
		return err
	}

	if turnOffConstUpdateResp.GetStatus() == homework_2.Status_USERNOTFOUND || turnOffConstUpdateResp.GetStatus() == homework_2.Status_MAILSERVICENOTFOUND {
		msg := tgbotapi.NewMessage(int64(user.chatId), "Something went wrong")
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
		return nil
	}

	mailService.constantlyUpdate = false
	isAnyServicesRemained := false
	for _, service := range user.mailServ {
		if service.constantlyUpdate {
			isAnyServicesRemained = true
		}
	}
	user.oneOrMoreConstUpdate = isAnyServicesRemained

	msg := tgbotapi.NewMessage(int64(user.chatId), "Successfully")
	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (tgBot *TelegramBot) handlerGetLastMessage(update *tgbotapi.Update) error {
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

	getLasMessageResp, err := tgBot.client.GetLastMessages(tgBot.ctx, &homework_2.GetLastMessageReq{
		Id:              1,
		AmountMessages:  1,
		MailServiceName: internalName,
		UserName:        userName,
		TelegramID:      int64(user.telegramId),
	})
	if err != nil {
		return err
	}
	fmt.Println(getLasMessageResp.GetStatus())
	if getLasMessageResp.GetStatus() == homework_2.Status_SUCCESS {
		lastMessages := getLasMessageResp.GetMessages()
		for _, message := range lastMessages {
			msg := tgbotapi.NewMessage(int64(user.chatId), message.GetUserName()+"\n"+message.GetMessageHeader()+"\n"+message.GetMessageBody())
			if _, err := tgBot.bot.Send(msg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (tgBot *TelegramBot) findUserInInternalDb(telegramId int) (telegramUser *TelegramUser, isFound bool) {
	if tgBot == nil {
		return nil, false
	}
	for _, user := range tgBot.users {
		if user.telegramId == telegramId {
			return user, true
		}
	}
	return nil, false
}

func (user *TelegramUser) findMailServiceInInternalDb(username, internalName string) (mailService *MailService, isFound bool) {
	if user == nil {
		return nil, false
	}
	for _, service := range user.mailServ {
		if service.userName == username && service.internalName == internalName {
			return service, true
		}
	}
	return nil, false
}

func (tgBot *TelegramBot) CloseConnectionWithServer() {
	tgBot.connectionToServer.Close()
}
