package main

import (
	"fmt"
	"github.com/emersion/go-imap/client"
	"gitlab.ozon.dev/VeneLooool/homework-2/internal/app"
	"log"
	"time"
)

func main() {
	err := error(nil)
	user := app.BuildUpUserMailService("imap.gmail.com:993", "timofej.gromb31", "fqeiywhhftilubhh")
	//user := app.BuildUpUserMailService("imap.yandex.ru:993", "timofej.gromb38", "Lop05rds/")

	if err = user.ConnectToService(); err != nil {
		log.Fatalln(err)
	}
	defer user.LogoutFromService()

	_, body, err := user.GetLastMessagesFromMailBox("INBOX")
	fmt.Println(body)
	if err != nil {
		log.Fatalln(err)
	}
	user.Data.Updates = make(chan client.Update)
	user.Data.NewUpdate = make(chan bool, 10)
	go user.CheckForUpdate("INBOX")
	flag := false
	for {

		if len(user.Data.NewUpdate) != 0 {
			flag = <-user.Data.NewUpdate
		}
		fmt.Println(flag)
		time.Sleep(1 * time.Second)
	}
}
