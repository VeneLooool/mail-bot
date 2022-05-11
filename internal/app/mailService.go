package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"io/ioutil"
	"log"
	"net/mail"
	"strings"
)

type UserMailService struct {
	nameMailServ string
	password     string
	username     string
	isLogin      bool
	client       *client.Client
	Data         ThisData
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
func (mailServ *UserMailService) CheckForUpdate(mailBoxName string) error {
	if _, err := mailServ.client.Select(mailBoxName, false); err != nil {
		return err
	}

	mailServ.client.Updates = mailServ.Data.Updates
	//stopped := false
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- mailServ.client.Idle(stop, nil)
	}()
	for {
		select {
		case update := <-mailServ.Data.Updates:
			log.Println("New update:", update)
			mailServ.Data.NewUpdate <- true
			/*if !stopped {
				close(stop)
				stopped = true
			}*/
		case err := <-done:
			if err != nil {
				return err
			}
			log.Println("Not idling anymore")
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

func (mailServ *UserMailService) GetLastMessagesFromMailBox(mailBoxName string) (header mail.Header, bodyMessage string, err error) {
	mailBox, err := mailServ.client.Select(mailBoxName, false)
	if err != nil {
		return nil, "", err
	}
	if mailBox.Messages == 0 {
		return nil, "", errors.New(fmt.Sprintf("no message in mail box: %s", mailBoxName))
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddRange(mailBox.Messages, mailBox.Messages)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}

	message := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- mailServ.client.Fetch(seqSet, items, message)
	}()
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

	body, err := ioutil.ReadAll(m.Body)
	if err != nil {
		return nil, "", err
	}

	cleanBody, err := cleanMessageBody(string(body))
	if err != nil {
		return nil, "", err
	}

	newBody, err := base64.StdEncoding.DecodeString(cleanBody)
	if err != nil {
		return nil, "", err
	}

	return m.Header, string(newBody), nil
}
func cleanMessageBody(messageBody string) (cleanBody string, err error) {
	lastIndexIndetificator := strings.Index(messageBody, "\n")
	if lastIndexIndetificator <= 0 {
		return "", errors.New("no indetificator was found")
	}
	indetificator := messageBody[:lastIndexIndetificator]
	messageBody = messageBody[lastIndexIndetificator+1:]
	base64Index := strings.Index(messageBody, "base64")
	if base64Index <= 0 {
		return messageBody, nil
	}
	lastIndexIndetificator = strings.Index(messageBody, indetificator)
	return messageBody[base64Index+7 : lastIndexIndetificator], nil
}
