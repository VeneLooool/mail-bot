package telegram

import (
	"context"
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	homework_2 "gitlab.ozon.dev/VeneLooool/homework-2/api"
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
	internalName     string
	externalName     string
	userName         string
	password         string
	constantlyUpdate bool
	constUpdateChan  chan bool
}

type TelegramUser struct {
	telegramId  int
	chatId      int
	mailServ    []*MailService
	updateMutex sync.Mutex
	update      tgbotapi.Update
	inputFlags  InputFlag
}

type TelegramBot struct {
	token                 string
	users                 []*TelegramUser
	bot                   *tgbotapi.BotAPI
	updatesConfig         tgbotapi.UpdateConfig
	updates               tgbotapi.UpdatesChannel
	client                homework_2.MailServClient
	connectionToServer    *grpc.ClientConn
	availableMailServices []string
}

func CreateTelegramBot(target, token string, timeout int) (telegramBot TelegramBot, err error) {
	telegramBot.availableMailServices = []string{"imap.gmail.com:993", "imap.yandex.ru:993"}
	telegramBot.token = token
	telegramBot.users = make([]*TelegramUser, 0)
	telegramBot.bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		return TelegramBot{}, err
	}
	telegramBot.updatesConfig = tgbotapi.NewUpdate(0)
	telegramBot.updatesConfig.Timeout = timeout
	telegramBot.updates, err = telegramBot.bot.GetUpdatesChan(telegramBot.updatesConfig)
	if err != nil {
		return TelegramBot{}, err
	}

	telegramBot.connectionToServer, err = grpc.Dial(target, grpc.WithInsecure())
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
			if strings.Index(update.CallbackQuery.Data, "ADDMAIL") != -1 {
				if err := tgBot.handlerAddMailService(&update); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (tgBot *TelegramBot) commandStart(update *tgbotapi.Update) error {
	_, isFound := tgBot.findUserInInternalDb(update.Message.From.ID)
	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx,
		"sender", "testClient",
		"when", time.Now().Format(time.RFC3339),
		"sender", "route256",
	)
	if isFound {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Choose one option")
		msg.ReplyMarkup = MainMenuButton
		if _, err := tgBot.bot.Send(msg); err != nil {
			return err
		}
		return nil
	}
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
		mailServNameInlineRow = append(mailServNameInlineRow, tgbotapi.NewInlineKeyboardButtonData(externalName, internalName+"_ADDMAIL"))
	}
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(mailServNameInlineRow)
	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}
	return nil
}

func (tgBot *TelegramBot) handlerAddMailService(update *tgbotapi.Update) error {
	user, isFound := tgBot.findUserInInternalDb(update.CallbackQuery.From.ID)
	if !isFound {
		return errors.New("unimplemented error")
	}
	user.inputFlags.username = true
	user.mailServ = append(user.mailServ, &MailService{
		internalName: update.CallbackQuery.Data[:strings.IndexRune(update.CallbackQuery.Data, '_')],
		externalName: update.CallbackQuery.Data[strings.IndexRune(update.CallbackQuery.Data, '.')+1 : strings.IndexRune(update.CallbackQuery.Data, ':')],
	})

	msg := tgbotapi.NewMessage(int64(user.chatId), "pls enter username:")
	msg.ReplyMarkup = nil
	if _, err := tgBot.bot.Send(msg); err != nil {
		return err
	}
	return nil
}
func (tgBot *TelegramBot) handlerDefaultMessage(update *tgbotapi.Update) error {
	return nil
}

func (tgBot *TelegramBot) findUserInInternalDb(telegramId int) (telegramUser *TelegramUser, isFound bool) {
	for _, user := range tgBot.users {
		if user.telegramId == telegramId {
			return user, true
		}
	}
	return nil, false
}

func (tgBot *TelegramBot) CloseConnectionWithServer() {
	tgBot.connectionToServer.Close()
}
