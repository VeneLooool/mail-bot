package main

import (
	"fmt"
	api2 "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"gitlab.ozon.dev/VeneLooool/homework-2/internal/app"
	"google.golang.org/grpc"
	"net"
	"time"
)

func main() {
	newServer := app.Server{}
	newServer.AddAvailableServices([]string{"imap.gmail.com:993", "imap.yandex.ru:993"})
	fmt.Println("Server has started")
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(app.ValidatorInterceptor),
	}
	grpcServer := grpc.NewServer(opts...)
	api2.RegisterMailServServer(grpcServer, &newServer)
	err = grpcServer.Serve(listener)
	if err != nil {
		panic(err)
	}
	for {
		time.Sleep(time.Millisecond * 100)
	}
}
