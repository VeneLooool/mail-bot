package pkg

import "gitlab.ozon.dev/VeneLooool/homework-2/internal/app"

type User struct {
	mailServices []app.UserMailService
	telegramId   string
}
