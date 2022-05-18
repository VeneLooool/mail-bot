package app

import (
	"errors"
	"fmt"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	enmime "github.com/jhillyerd/go.enmime"
	homework_2 "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"log"
	"net/mail"
	"sync"
	"time"
)

type UserMailService struct {
	nameMailServ     string
	password         string
	username         string
	isLogin          bool
	constantlyUpdate bool
	constUpdateChan  chan bool
	client           *client.Client
	Data             ThisData
}
type ThisData struct {
	mailboxes chan *imap.MailboxInfo
	Updates   chan client.Update
	NewUpdate chan bool
}

func BuildUpUserMailService(nameMailServ, username, password string) UserMailService {
	var mailService UserMailService
	mailService.nameMailServ = nameMailServ
	mailService.password = password
	mailService.username = username
	return mailService
}

//TODO поправить возможно убрать data
func (mailServ *UserMailService) RefreshMailBoxes() error {
	//TODO тут должно стоять не 10, а количетсво mailBoxes, хз как получить их
	if mailServ.Data.mailboxes == nil {
		mailServ.Data.mailboxes = make(chan *imap.MailboxInfo, 10)
	}
	done := make(chan error, 1)
	go func() {
		done <- mailServ.client.List("", "*", mailServ.Data.mailboxes)
	}()
	if err := <-done; err != nil {
		return err
	}
	return nil
}
func (mailServ *UserMailService) GetAllMailBoxes() (result []imap.MailboxInfo, err error) {
	if mailServ.Data.mailboxes == nil {
		if err = mailServ.RefreshMailBoxes(); err != nil {
			return nil, err
		}

	}

	for value := range mailServ.Data.mailboxes {
		result = append(result, *value)
	}

	return result, nil
}
func (mailServ *UserMailService) CheckForUpdate(user *User, mutexUpdate *sync.Mutex) error {
	var waitGroup sync.WaitGroup
	mailBox, err := mailServ.client.Select("INBOX", false)
	if err != nil {
		return err
	}

	mailServ.client.Updates = mailServ.Data.Updates
	stop := make(chan struct{})
	done := make(chan error, 1)
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		done <- mailServ.client.Idle(stop, nil)
	}()
	for {
		select {
		case <-mailServ.Data.Updates:
			close(stop)
			waitGroup.Wait()
			_, body, err := mailServ.GetLastMessagesFromMailBox(mailBox)
			if err != nil {
				panic(err)
			}
			message := &homework_2.MailMessages{
				MailServiceName: mailServ.nameMailServ,
				UserName:        mailServ.username,
				MessageHeader:   "",
				MessageBody:     body,
			}
			mutexUpdate.Lock()
			user.updates = append(user.updates, message)
			mutexUpdate.Unlock()
			time.Sleep(time.Second * 50)
			stop = make(chan struct{})
			done = make(chan error, 1)
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				done <- mailServ.client.Idle(stop, nil)
			}()
		case constUpdate := <-mailServ.constUpdateChan:
			fmt.Println("Я закрылся")
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

func (mailServ *UserMailService) ConnectToService() error {
	err := error(nil)

	mailServ.client, err = client.DialTLS(mailServ.nameMailServ, nil)
	if err != nil {
		return err
	}

	log.Printf("Succecfule connected to server for user: %s\n", mailServ.username)

	if err = mailServ.LoginToService(); err != nil {
		return err
	}

	log.Printf("Succecfule login to server for user: %s\n", mailServ.username)

	return nil
}
func (mailServ *UserMailService) LoginToService() error {
	if mailServ.client == nil {
		return errors.New("user isn't connected to the server")
	}
	if err := mailServ.client.Login(mailServ.username, mailServ.password); err != nil {
		return err
	}
	mailServ.isLogin = true
	return nil
}
func (mailServ *UserMailService) LogoutFromService() error {
	err := mailServ.client.Logout()
	if err != nil {
		return err
	}
	mailServ.client = nil
	mailServ.isLogin = false
	log.Printf("Seccecfule logout for user: %s\n", mailServ.username)
	return nil
}

//TODO должно быть n количество последних сообщений
func (mailServ *UserMailService) GetLastMessagesFromMailBox(mailBox *imap.MailboxStatus) (header mail.Header, bodyMessage string, err error) {

	if mailBox.Messages == 0 {
		return nil, "", nil
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddRange(mailBox.Messages, mailBox.Messages)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}

	message := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	//waitFor.Add(1)
	//go func() {
	//defer waitFor.Done()
	done <- mailServ.client.Fetch(seqSet, items, message)
	//}()
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

	log.Println(mime.Text)
	return m.Header, mime.Text, nil
}

func (mailServ *UserMailService) IsConstantlyUpdate() bool {
	return mailServ.constantlyUpdate
}
func (mailServ *UserMailService) GetNameForMailService() string {
	return mailServ.nameMailServ
}
func (mailServ *UserMailService) GetUsername() string {
	return mailServ.username
}
func (mailServ *UserMailService) GetSwitchConstantlyUpdate() bool {
	return mailServ.constantlyUpdate
}
func (mailServ *UserMailService) UpdateSwitchConstantlyUpdate(switcher bool) {
	mailServ.constantlyUpdate = switcher
}
