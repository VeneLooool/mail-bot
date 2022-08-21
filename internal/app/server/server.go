package server

import (
	api2 "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"gitlab.ozon.dev/VeneLooool/homework-2/config"
	"gitlab.ozon.dev/VeneLooool/homework-2/internal/app/server/service/email"
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
	service      []*email.Service
	Updates      []*api2.MailMessages
	updatesMutex sync.Mutex
}

type Server struct {
	users []*User
	api2.UnimplementedMailServServer
	services []string
}

func NewServer(config config.Config) *Server {
	return &Server{
		services: config.GetMailServices(),
	}
}
func Interceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if v, ok := req.(interface{ GetId() int32 }); ok {
		if v.GetId() < 0 {
			return nil, status.Error(codes.InvalidArgument, "Bad Id")
		}
	} else {
		return nil, status.Error(codes.InvalidArgument, "Bad Request")
	}
	return handler(ctx, req)
}

func (serv *Server) CreateUser(ctx context.Context, req *api2.CreateUserReq) (*api2.CreateUserResp, error) {
	if _, isFound := serv.findUserInDB(req.GetTelegramID()); !isFound {
		serv.users = append(serv.users, &User{
			telegramID: req.GetTelegramID(),
			service:    make([]*email.Service, 0),
			Updates:    make([]*api2.MailMessages, 0),
		})
	}

	return &api2.CreateUserResp{
		Id:         req.GetId(),
		Status:     api2.Status_SUCCESS,
		TelegramID: req.GetTelegramID(),
	}, nil
}
func (serv *Server) CustomerSearch(ctx context.Context, req *api2.CustomerSearchReq) (resp *api2.CustomerSearchResp, err error) {
	resp = &api2.CustomerSearchResp{
		Id:         req.GetId(),
		TelegramID: req.GetTelegramID(),
	}

	if _, ok := serv.findUserInDB(req.GetTelegramID()); ok {
		resp.Status = api2.Status_SUCCESS
	} else {
		resp.Status = api2.Status_USERNOTFOUND
	}
	return resp, nil
}
func (serv *Server) DeleteUser(ctx context.Context, req *api2.DeleteUserReq) (resp *api2.DeleteUserResp, err error) {
	resp = &api2.DeleteUserResp{
		Id:         req.GetId(),
		Stratus:    api2.Status_SUCCESS,
		TelegramID: req.GetTelegramID(),
	}
	return resp, nil
}

func (serv *Server) AddNewMailService(ctx context.Context, req *api2.AddNewMailServiceReq) (resp *api2.AddNewMailServiceResp, err error) {
	resp = &api2.AddNewMailServiceResp{
		Id:         req.GetId(),
		TelegramID: req.GetTelegramID(),
	}

	user, isFound := serv.findUserInDB(req.GetTelegramID())
	if !isFound {
		resp.Status = api2.Status_USERNOTFOUND
		return resp, nil
	}

	service, isFound := user.findService(req.GetMailServiceName(), req.GetLogin())
	if !isFound || service.GetUsername() != req.GetLogin() {
		newService := email.NewUserService(req.GetMailServiceName(), req.GetLogin(), req.GetPassword())

		if err := newService.Connect(); err != nil {
			resp.Status = api2.Status_INCORRECTLOGINDATA
			return resp, nil
		}
		user.service = append(user.service, &newService)
	}
	resp.Status = api2.Status_SUCCESS
	return resp, nil
}
func (serv *Server) ConstantlyUpdate(ctx context.Context, req *api2.ConstantlyUpdateReq) (resp *api2.ConstantlyUpdateResp, err error) {
	resp = &api2.ConstantlyUpdateResp{
		Id:         req.GetId(),
		TelegramID: req.GetTelegramID(),
	}
	user, isFound := serv.findUserInDB(req.GetTelegramID())
	if !isFound {
		resp.Status = api2.Status_USERNOTFOUND
		return resp, nil
	}
	service, isFound := user.findService(req.GetMailServiceName(), req.GetLogin())
	if !isFound {
		resp.Status = api2.Status_MAILSERVICENOTFOUND
		return resp, nil
	}

	if req.GetSwitch() != service.GetOnlineUpdateSwitch() {
		if req.GetSwitch() {
			service.OpenUpdateChan()
			service.SetOnlineUpdateSwitch(true)
			go service.UpdateHandler(user, &user.updatesMutex)
		} else {
			service.CloseUpdateChan()
			service.SetOnlineUpdateSwitch(false)
		}
	}
	resp.Status = api2.Status_SUCCESS
	return resp, nil

}
func (serv *Server) CheckForUpdates(ctx context.Context, req *api2.CheckForUpdatesReq) (resp *api2.CheckForUpdatesResp, err error) {
	resp = &api2.CheckForUpdatesResp{
		Id:         req.GetId(),
		TelegramID: req.GetTelegramID(),
	}

	user, isFound := serv.findUserInDB(req.GetTelegramID())
	if !isFound {
		resp.Status = api2.Status_USERNOTFOUND
		return resp, nil
	}

	user.updatesMutex.Lock()
	defer user.updatesMutex.Unlock()
	if len(user.Updates) != 0 {
		resp.Status = api2.Status_SUCCESS
		resp.Messages = user.Updates
		user.Updates = make([]*api2.MailMessages, 0)
	} else {
		resp.Status = api2.Status_NOUPDATES
	}
	return resp, nil
}
func (serv *Server) GetLastMessages(ctx context.Context, req *api2.GetLastMessageReq) (resp *api2.GetLastMessageResp, err error) {
	resp = &api2.GetLastMessageResp{
		Id:         req.GetId(),
		TelegramID: req.GetTelegramID(),
	}

	user, isFound := serv.findUserInDB(req.GetTelegramID())
	if !isFound {
		resp.Status = api2.Status_USERNOTFOUND
		return resp, nil
	}
	if req.GetAmountMessages() != 1 {
		resp.Status = api2.Status_NOTENOUGHMESSAGES
		return resp, nil
	}
	service, isFound := user.findService(req.GetMailServiceName(), req.GetUserName())
	if !isFound {
		resp.Status = api2.Status_MAILSERVICENOTFOUND
		return resp, nil
	}
	if service.GetOnlineUpdateSwitch() {
		resp.Status = api2.Status_FAIL
		return resp, nil
	}
	mailBox, err := service.SetMailBoxes("INBOX")
	if err != nil {
		return nil, err
	}
	_, messBody, err := service.GetLastMessages(mailBox)
	if err != nil {
		return nil, err
	}
	messages := make([]*api2.MailMessages, 0)
	message := &api2.MailMessages{
		MailServiceName: req.GetMailServiceName(),
		UserName:        req.GetUserName(),
		MessageHeader:   "",
		MessageBody:     messBody,
	}
	messages = append(messages, message)
	resp.Status = api2.Status_SUCCESS
	resp.Messages = messages
	return resp, nil
}
func (serv *Server) GetListAvailableMailServices(ctx context.Context, req *api2.GetListAvailableMailServicesReq) (resp *api2.GetListAvailableMailServicesResp, err error) {
	resp = &api2.GetListAvailableMailServicesResp{
		Id:         req.GetId(),
		TelegramID: req.GetTelegramID(),
	}

	user, isFound := serv.findUserInDB(req.GetTelegramID())
	if !isFound {
		resp.Status = api2.Status_USERNOTFOUND
		return resp, nil
	}

	availableServices := make([]*api2.MailService, 0)
	for _, name := range serv.services {
		services, isFound := user.findServices(name)
		if isFound {
			for _, service := range services {
				externalRep := service.GetName()
				externalRep = externalRep[strings.IndexRune(externalRep, '.'):strings.IndexRune(externalRep, ':')]
				availableServices = append(availableServices, &api2.MailService{
					UserName:                   service.GetUsername(),
					MailServiceNameInternalRep: service.GetName(),
					MailServiceNameExternalRep: externalRep,
				})
			}
		}
	}
	if len(availableServices) == 0 {
		resp.Status = api2.Status_FAIL
	} else {
		resp.Status = api2.Status_SUCCESS
		resp.AvailableMailServices = availableServices
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

func (user *User) findServices(name string) (pointer []*email.Service, isFound bool) {
	if user == nil {
		return nil, false
	}
	pointer = make([]*email.Service, 0)
	for _, mailServ := range user.service {
		if mailServ.GetName() == name {
			pointer = append(pointer, mailServ)
			isFound = true
		}
	}
	if isFound {
		return pointer, isFound
	}
	return nil, isFound
}
func (user *User) findService(serviceName, username string) (pointer *email.Service, isFound bool) {
	if user == nil {
		return nil, false
	}
	allPointers, ok := user.findServices(serviceName)
	if !ok {
		return nil, false
	}
	for i, p := range allPointers {
		if p.GetUsername() == username {
			return allPointers[i], true
		}
	}
	return nil, false
}
