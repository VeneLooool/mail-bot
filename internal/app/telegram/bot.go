package telegram

import (
	"context"
	"fmt"
	botApi "github.com/go-telegram-bot-api/telegram-bot-api"
	api2 "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"gitlab.ozon.dev/VeneLooool/homework-2/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"strings"
	"sync"
	"time"
)

type InputFlag struct {
	username bool
	password bool
}

type MailService struct {
	internalName string
	externalName string
	userName     string
	password     string
	onlineUpdate bool
	isLogin      bool
}

type TelegramUser struct {
	telegramId      int
	chatId          int
	mailServ        []*MailService
	updateMutex     sync.Mutex
	update          botApi.Update
	inputFlags      InputFlag
	userMutex       sync.Mutex
	hasOnlineUpdate bool
}

type TelegramBot struct {
	token              string
	users              []*TelegramUser
	botMutex           sync.Mutex
	bot                *botApi.BotAPI
	updatesConfig      botApi.UpdateConfig
	updates            botApi.UpdatesChannel
	client             api2.MailServClient
	clientMutex        sync.Mutex
	connectionToServer *grpc.ClientConn
	ctx                context.Context
	services           []string
}

func NewTelegramBot(timeout int) (telegramBot TelegramBot, err error) {
	configuration, err := config.GetConfig()
	if err != nil {
		return TelegramBot{}, err
	}

	telegramBot.services = configuration.GetMailServices()
	telegramBot.token = configuration.GetTelegramToken()

	telegramBot.users = make([]*TelegramUser, 0)
	telegramBot.ctx = context.Background()

	telegramBot.ctx = metadata.AppendToOutgoingContext(telegramBot.ctx,
		"sender", "Client",
		"when", time.Now().Format(time.RFC3339),
		"sender", "route256",
	)

	telegramBot.bot, err = botApi.NewBotAPI(telegramBot.token)
	if err != nil {
		return TelegramBot{}, err
	}

	telegramBot.updatesConfig = botApi.NewUpdate(0)
	telegramBot.updatesConfig.Timeout = timeout

	telegramBot.updates, err = telegramBot.bot.GetUpdatesChan(telegramBot.updatesConfig)
	if err != nil {
		return TelegramBot{}, err
	}

	telegramBot.connectionToServer, err = grpc.Dial("localhost"+configuration.GetAddressPort(), grpc.WithInsecure())
	if err != nil {
		return TelegramBot{}, nil
	}

	telegramBot.client = api2.NewMailServClient(telegramBot.connectionToServer)
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
				if err := tgBot.commandAddService(&update); err != nil {
					return err
				}
			case "Go to mail service":
				if err := tgBot.commandGoToMailService(&update); err != nil {
					return err
				}
			case "Synchronize your mail service with main serv":
				if err := tgBot.commandSyncServices(&update); err != nil {
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
