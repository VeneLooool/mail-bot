package email

import (
	"errors"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	enmime "github.com/jhillyerd/go.enmime"
	api "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"gitlab.ozon.dev/VeneLooool/homework-2/internal/app/server"
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
	email             email
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

func (service *Service) refreshMail() error {
	if service.email.boxes == nil {
		service.email.boxes = make(chan *imap.MailboxInfo, 10)
	}
	done := make(chan error, 1)
	go func() {
		done <- service.client.List("", "*", service.email.boxes)
	}()
	if err := <-done; err != nil {
		return err
	}
	return nil
}
func (service *Service) GetMailBoxes() (boxes []imap.MailboxInfo, err error) {
	if service.email.boxes == nil {
		if err = service.refreshMail(); err != nil {
			return nil, err
		}
	}

	for box := range service.email.boxes {
		boxes = append(boxes, *box)
	}
	return boxes, nil
}
func (service *Service) SetMailBoxes(name string) (*imap.MailboxStatus, error) {
	return service.client.Select(name, false)
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
func (service *Service) UpdateHandler(user *server.User, mutexUpdate *sync.Mutex) error {
	var waitGroup sync.WaitGroup

	mailBox, err := service.client.Select("INBOX", false)
	if err != nil {
		return err
	}

	service.client.Updates = service.email.Updates
	stop := make(chan struct{})
	done := make(chan error, 1)
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		done <- service.client.Idle(stop, nil)
	}()

	for {
		select {
		case <-service.email.Updates:
			close(stop)
			waitGroup.Wait()

			_, body, err := service.GetLastMessages(mailBox)
			if err != nil {
				panic(err)
			}

			message := service.NewMessage("", body)
			mutexUpdate.Lock()
			user.Updates = append(user.Updates, message)
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

func (service *Service) NewMessage(header string, body string) *api.MailMessages {
	return &api.MailMessages{
		MailServiceName: service.name,
		UserName:        service.username,
		MessageHeader:   header,
		MessageBody:     body,
	}
}

func (service *Service) GetName() string {
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
func (service *Service) OpenUpdateChan() {
	service.onlineUpdatesChan = make(chan bool, 10)
	service.email.Updates = make(chan client.Update)
}
func (service *Service) CloseUpdateChan() {
	service.onlineUpdatesChan <- false
	close(service.onlineUpdatesChan)
}
