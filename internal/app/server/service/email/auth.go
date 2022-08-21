package email

import (
	"errors"
	"github.com/emersion/go-imap/client"
)

func (service *Service) Connect() (err error) {
	if service.client, err = client.DialTLS(service.name, nil); err != nil {
		return err
	}
	if err = service.Login(); err != nil {
		return err
	}
	return nil
}
func (service *Service) Login() error {
	if service.client == nil {
		return errors.New("user isn't connected to the server")
	}
	if err := service.client.Login(service.username, service.password); err != nil {
		return err
	}
	service.isLogin = true
	return nil
}
func (service *Service) Logout() error {
	if err := service.client.Logout(); err != nil {
		return err
	}
	service.client = nil
	service.isLogin = false
	return nil
}
