package server

import (
	"fmt"
	"github.com/emersion/go-imap/client"
	api2 "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"sync"
)

type Message struct {
	serviceName string
	username    string
	header      string
	body        string
}

type User struct {
	telegramID   int64
	mailServices []*UserMailService
	updates      []*api2.MailMessages
	updatesMutex sync.Mutex
}

type Server struct {
	users []*User
	api2.UnimplementedMailServServer
	availableMailServicesName []string
}

//TODO почти везде не проверяется на то что установленно ли connection с сервисом для юзера(исправить)
func ValidatorInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if v, ok := req.(interface{ GetId() int32 }); ok {
		if v.GetId() < 0 {
			return nil, status.Error(codes.InvalidArgument, "Bad Id")
		}
	} else {
		return nil, status.Error(codes.InvalidArgument, "Bad Request")
	}
	fmt.Println("Had worked interceptor!")
	return handler(ctx, req)
}

func (serv *Server) CreateUser(ctx context.Context, req *api2.CreateUserReq) (resp *api2.CreateUserResp, err error) {
	if _, ok := serv.findUserInDB(req.GetTelegramID()); ok {
		resp = &api2.CreateUserResp{
			Id:         req.GetId(),
			Status:     api2.Status_SUCCESS,
			TelegramID: req.GetTelegramID(),
		}
		return resp, nil
	}
	serv.users = append(serv.users, &User{
		telegramID:   req.GetTelegramID(),
		mailServices: make([]*UserMailService, 0),
		updates:      make([]*api2.MailMessages, 0),
	})
	fmt.Printf("Create user where tgID:%d\n", req.GetTelegramID())
	resp = &api2.CreateUserResp{
		Id:         req.GetId(),
		Status:     api2.Status_SUCCESS,
		TelegramID: req.GetTelegramID(),
	}
	return resp, nil
}
func (serv *Server) CustomerSearch(ctx context.Context, req *api2.CustomerSearchReq) (resp *api2.CustomerSearchResp, err error) {
	if _, ok := serv.findUserInDB(req.GetTelegramID()); ok {
		resp = &api2.CustomerSearchResp{
			Id:         req.GetId(),
			Status:     api2.Status_SUCCESS,
			TelegramID: req.GetTelegramID(),
		}
		return resp, nil
	}
	resp = &api2.CustomerSearchResp{
		Id:         req.GetId(),
		Status:     api2.Status_USERNOTFOUND,
		TelegramID: req.GetTelegramID(),
	}
	return resp, nil
}
func (serv *Server) DeleteUser(ctx context.Context, req *api2.DeleteUserReq) (resp *api2.DeleteUserResp, err error) {
	//TODO тут просто заглушка переделать
	resp = &api2.DeleteUserResp{Id: req.GetId(), Stratus: api2.Status_SUCCESS, TelegramID: req.GetTelegramID()}
	return resp, nil
}

func (serv *Server) AddNewMailService(ctx context.Context, req *api2.AddNewMailServiceReq) (resp *api2.AddNewMailServiceResp, err error) {
	user, isFound := serv.findUserInDB(req.GetTelegramID())
	if !isFound {
		resp = &api2.AddNewMailServiceResp{
			Id:         req.GetId(),
			Status:     api2.Status_USERNOTFOUND,
			TelegramID: req.GetTelegramID(),
		}
		return resp, nil
	}
	mailService, isFound := user.findMailService(req.GetMailServiceName(), req.GetLogin())
	if isFound {
		if mailService.GetUsername() == req.GetLogin() {
			resp = &api2.AddNewMailServiceResp{
				Id:         req.GetId(),
				Status:     api2.Status_SUCCESS,
				TelegramID: req.GetTelegramID(),
			}
			return resp, nil
		}
	}
	newMailService := BuildUpUserMailService(req.GetMailServiceName(), req.GetLogin(), req.GetPassword())
	if err := newMailService.ConnectToService(); err != nil {
		resp = &api2.AddNewMailServiceResp{
			Id:         req.GetId(),
			Status:     api2.Status_INCORRECTLOGINDATA,
			TelegramID: req.GetTelegramID(),
		}
		return resp, nil
	}
	user.mailServices = append(user.mailServices, &newMailService)
	resp = &api2.AddNewMailServiceResp{
		Id:         req.GetId(),
		Status:     api2.Status_SUCCESS,
		TelegramID: req.GetTelegramID(),
	}
	return resp, nil
}

//TODO проработать закрытие открытие запись каналов(мне кажется где-то есть ошибка)
func (serv *Server) ConstantlyUpdate(ctx context.Context, req *api2.ConstantlyUpdateReq) (resp *api2.ConstantlyUpdateResp, err error) {
	user, isFound := serv.findUserInDB(req.GetTelegramID())
	if !isFound {
		resp = &api2.ConstantlyUpdateResp{
			Id:         req.GetId(),
			Status:     api2.Status_USERNOTFOUND,
			TelegramID: req.GetTelegramID(),
		}
		return resp, nil
	}
	mailService, isFound := user.findMailService(req.GetMailServiceName(), req.GetLogin())
	if !isFound {
		resp = &api2.ConstantlyUpdateResp{
			Id:         req.GetId(),
			Status:     api2.Status_MAILSERVICENOTFOUND,
			TelegramID: req.GetTelegramID(),
		}
		return resp, nil
	}

	if req.GetSwitch() == mailService.GetSwitchConstantlyUpdate() {
		resp = &api2.ConstantlyUpdateResp{
			Id:         req.GetId(),
			Status:     api2.Status_SUCCESS,
			TelegramID: req.GetTelegramID(),
		}
		return resp, nil
	} else {
		if req.GetSwitch() && !mailService.GetSwitchConstantlyUpdate() {
			mailService.constUpdateChan = make(chan bool, 10)
			mailService.Data.Updates = make(chan client.Update)
			mailService.UpdateSwitchConstantlyUpdate(true)
			go mailService.CheckForUpdate(user, &user.updatesMutex)
		} else {
			mailService.constUpdateChan <- false
			mailService.constantlyUpdate = false
			close(mailService.constUpdateChan)
		}
	}
	resp = &api2.ConstantlyUpdateResp{
		Id:         req.GetId(),
		Status:     api2.Status_SUCCESS,
		TelegramID: req.GetTelegramID(),
	}

	return resp, nil

}

func (serv *Server) CheckForUpdates(ctx context.Context, req *api2.CheckForUpdatesReq) (resp *api2.CheckForUpdatesResp, err error) {
	user, isFound := serv.findUserInDB(req.GetTelegramID())
	if !isFound {
		resp = &api2.CheckForUpdatesResp{
			Id:         req.GetId(),
			Status:     api2.Status_USERNOTFOUND,
			TelegramID: req.GetTelegramID(),
		}
		return resp, nil
	}
	user.updatesMutex.Lock()
	if len(user.updates) != 0 {
		resp = &api2.CheckForUpdatesResp{
			Id:         req.GetId(),
			Status:     api2.Status_SUCCESS,
			Messages:   user.updates,
			TelegramID: req.GetTelegramID(),
		}
		user.updates = make([]*api2.MailMessages, 0)
	} else {
		resp = &api2.CheckForUpdatesResp{
			Id:         req.GetId(),
			Status:     api2.Status_NOUPDATES,
			Messages:   nil,
			TelegramID: req.GetTelegramID(),
		}
	}
	user.updatesMutex.Unlock()
	return resp, nil
}

func (serv *Server) GetLastMessages(ctx context.Context, req *api2.GetLastMessageReq) (resp *api2.GetLastMessageResp, err error) {
	user, isFound := serv.findUserInDB(req.GetTelegramID())
	if !isFound {
		resp = &api2.GetLastMessageResp{
			Id:         req.GetId(),
			Status:     api2.Status_USERNOTFOUND,
			Messages:   nil,
			TelegramID: req.GetTelegramID(),
		}
		return resp, nil
	}
	if req.GetAmountMessages() != 1 {
		resp = &api2.GetLastMessageResp{
			Id:         req.GetId(),
			Status:     api2.Status_NOTENOUGHMESSAGES,
			Messages:   nil,
			TelegramID: req.GetTelegramID(),
		}
		return resp, nil
	}
	mailService, isFound := user.findMailService(req.GetMailServiceName(), req.GetUserName())
	if !isFound {
		resp = &api2.GetLastMessageResp{
			Id:         req.GetId(),
			Status:     api2.Status_MAILSERVICENOTFOUND,
			Messages:   nil,
			TelegramID: req.GetTelegramID(),
		}
		return resp, nil
	}
	if mailService.GetSwitchConstantlyUpdate() {
		resp = &api2.GetLastMessageResp{
			Id:         req.GetId(),
			Status:     api2.Status_FAIL,
			Messages:   nil,
			TelegramID: req.GetTelegramID(),
		}
		return resp, nil
	}
	mailBox, err := mailService.client.Select("INBOX", false)
	if err != nil {
		return nil, err
	}
	_, messBody, err := mailService.GetLastMessagesFromMailBox(mailBox)
	if err != nil {
		return nil, err
	}
	messagesArray := make([]*api2.MailMessages, 0)
	message := &api2.MailMessages{
		MailServiceName: req.GetMailServiceName(),
		UserName:        req.GetUserName(),
		MessageHeader:   "",
		MessageBody:     messBody,
	}
	messagesArray = append(messagesArray, message)
	resp = &api2.GetLastMessageResp{
		Id:         req.GetId(),
		Status:     api2.Status_SUCCESS,
		Messages:   messagesArray,
		TelegramID: req.GetTelegramID(),
	}
	return resp, nil
}

func (serv *Server) GetListAvailableMailServices(ctx context.Context, req *api2.GetListAvailableMailServicesReq) (resp *api2.GetListAvailableMailServicesResp, err error) {
	user, isFound := serv.findUserInDB(req.GetTelegramID())
	if !isFound {
		resp = &api2.GetListAvailableMailServicesResp{
			Id:                    req.GetId(),
			Status:                api2.Status_USERNOTFOUND,
			AvailableMailServices: nil,
			TelegramID:            req.GetTelegramID(),
		}
		return resp, nil
	}
	isFundAny := false
	availableMailServicesForUser := make([]*api2.MailService, 0)
	for _, name := range serv.availableMailServicesName {
		mailServices, isFound := user.findAllMailServices(name)
		if isFound {
			isFundAny = true
			for _, service := range mailServices {
				externalRep := service.GetNameForMailService()
				externalRep = externalRep[strings.IndexRune(externalRep, '.'):strings.IndexRune(externalRep, ':')]
				supportService := &api2.MailService{
					UserName:                   service.GetUsername(),
					MailServiceNameInternalRep: service.GetNameForMailService(),
					MailServiceNameExternalRep: externalRep,
				}
				availableMailServicesForUser = append(availableMailServicesForUser, supportService)
			}
		}
	}
	if !isFundAny {
		resp = &api2.GetListAvailableMailServicesResp{
			Id:                    req.GetId(),
			Status:                api2.Status_FAIL,
			AvailableMailServices: nil,
			TelegramID:            req.GetTelegramID(),
		}
		return resp, nil
	}
	resp = &api2.GetListAvailableMailServicesResp{
		Id:                    req.GetId(),
		Status:                api2.Status_SUCCESS,
		AvailableMailServices: availableMailServicesForUser,
		TelegramID:            req.GetTelegramID(),
	}
	return resp, nil
}

func (serv *Server) findUserInDB(telegramID int64) (user *User, isFound bool) {
	for i := range serv.users {
		if serv.users[i].telegramID == telegramID {
			return serv.users[i], true
		}
	}
	return nil, false
}

//TODO возможно ссылка не правильная будет(проверить)
func (user *User) findAllMailServices(mailServiceName string) (pointer []*UserMailService, isFound bool) {
	if user == nil {
		return nil, false
	}
	pointer = make([]*UserMailService, 0)
	for _, mailServ := range user.mailServices {
		if mailServ.nameMailServ == mailServiceName {
			pointer = append(pointer, mailServ)
			isFound = true
		}
	}
	if isFound {
		return pointer, isFound
	}
	return nil, isFound
}

func (user *User) findMailService(mailServiceName, username string) (pointer *UserMailService, isFound bool) {
	if user == nil {
		return nil, false
	}
	allPointers, ok := user.findAllMailServices(mailServiceName)
	if !ok {
		return nil, false
	}
	for i, p := range allPointers {
		if p.username == username {
			return allPointers[i], true
		}
	}
	return nil, false
}

func (serv *Server) AddAvailableServices(nameServices []string) {
	if serv == nil {
		return
	}
	for _, name := range nameServices {
		serv.availableMailServicesName = append(serv.availableMailServicesName, name)
	}
}
