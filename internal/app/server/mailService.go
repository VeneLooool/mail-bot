package server

import (
	"errors"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	enmime "github.com/jhillyerd/go.enmime"
	api "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"net/mail"
	"sync"
	"time"
)

type Service struct {
	name              string
	password          string
	username          string
	isLogin           bool
	onlineUpdates     bool
	onlineUpdatesChan chan bool
	client            *client.Client
	Email             email
}
type email struct {
	boxes     chan *imap.MailboxInfo
	Updates   chan client.Update
	NewUpdate chan bool
}

func NewUserService(name, username, password string) Service {
	return Service{
		name:     name,
		password: password,
		username: username,
	}
}

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

func (service *Service) RefreshMail() error {
	if service.Email.boxes == nil {
		service.Email.boxes = make(chan *imap.MailboxInfo, 10)
	}
	done := make(chan error, 1)
	go func() {
		done <- service.client.List("", "*", service.Email.boxes)
	}()
	if err := <-done; err != nil {
		return err
	}
	return nil
}
func (service *Service) GetMailBoxes() (result []imap.MailboxInfo, err error) {
	if service.Email.boxes == nil {
		if err = service.RefreshMail(); err != nil {
			return nil, err
		}
	}

	for value := range service.Email.boxes {
		result = append(result, *value)
	}
	return result, nil
}
func (service *Service) CheckForUpdate(user *User, mutexUpdate *sync.Mutex) error {
	var waitGroup sync.WaitGroup

	mailBox, err := service.client.Select("INBOX", false)
	if err != nil {
		return err
	}

	service.client.Updates = service.Email.Updates
	stop := make(chan struct{})
	done := make(chan error, 1)
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		done <- service.client.Idle(stop, nil)
	}()
	for {
		select {
		case <-service.Email.Updates:
			close(stop)
			waitGroup.Wait()

			_, body, err := service.GetLastMessages(mailBox)
			if err != nil {
				panic(err)
			}

			message := service.NewMessage("", body)
			mutexUpdate.Lock()
			user.updates = append(user.updates, message)
			mutexUpdate.Unlock()

			time.Sleep(time.Second * 50)
			stop = make(chan struct{})
			done = make(chan error, 1)
			waitGroup.Add(1)

			go func() {
				defer waitGroup.Done()
				done <- service.client.Idle(stop, nil)
			}()
		case constUpdate := <-service.onlineUpdatesChan:
			if !constUpdate {
				close(stop)
				return nil
			}
		case err := <-done:
			if err != nil {
				panic(err)
				return err
			}
		}
	}
}

func (service *Service) GetLastMessages(mailBox *imap.MailboxStatus) (header mail.Header, bodyMessage string, err error) {
	if mailBox.Messages == 0 {
		return nil, "", nil
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddRange(mailBox.Messages, mailBox.Messages)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}

	message := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	done <- service.client.Fetch(seqSet, items, message)

	msg := <-message
	r := msg.GetBody(section)

	if r == nil {
		return nil, "", errors.New("server didn't return message body")
	}
	if err = <-done; err != nil {
		return nil, "", err
	}

	m, err := mail.ReadMessage(r)
	if err != nil {
		return nil, "", err
	}
	mime, _ := enmime.ParseMIMEBody(&mail.Message{Header: m.Header, Body: m.Body})

	return m.Header, mime.Text, nil
}

func (service *Service) NewMessage(header string, body string) *api.MailMessages {
	return &api.MailMessages{
		MailServiceName: service.name,
		UserName:        service.username,
		MessageHeader:   header,
		MessageBody:     body,
	}
}

func (service *Service) GetNameForMailService() string {
	return service.name
}
func (service *Service) GetUsername() string {
	return service.username
}
func (service *Service) GetOnlineUpdateSwitch() bool {
	return service.onlineUpdates
}
func (service *Service) SetOnlineUpdateSwitch(switcher bool) {
	service.onlineUpdates = switcher
}
