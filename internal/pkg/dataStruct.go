package pkg

import (
	"gitlab.ozon.dev/VeneLooool/homework-2/internal/app/server"
)

type User struct {
	mailServices []server.UserMailService
	telegramId   string
}
