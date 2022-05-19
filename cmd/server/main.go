package main

import (
	"context"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	api2 "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"gitlab.ozon.dev/VeneLooool/homework-2/config"
	"gitlab.ozon.dev/VeneLooool/homework-2/internal/app/server"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
)

func runRest() {
	configuration, err := config.GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	mux := runtime.NewServeMux()
	//"localhost:8080"
	err = api2.RegisterMailServHandlerFromEndpoint(context.Background(), mux, configuration.GetServerAddressAndPort(), []grpc.DialOption{grpc.WithInsecure()})
	if err != nil {
		log.Fatal(err)
	}
	server := http.Server{
		Handler: mux,
	}

	l, err := net.Listen(configuration.GetHttpServNetwork(), configuration.GetHttpServAddress())
	if err != nil {
		log.Fatal(err)
	}
	err = server.Serve(l)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	configuration, err := config.GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	newServer := server.Server{}

	newServer.AddAvailableServices(configuration.GetAvailableMailServices())

	fmt.Println("Server has started")

	listener, err := net.Listen(configuration.GetServerNetwork(), configuration.GetServerAddressAndPort())
	if err != nil {
		panic(err)
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(server.ValidatorInterceptor),
	}
	grpcServer := grpc.NewServer(opts...)
	api2.RegisterMailServServer(grpcServer, &newServer)

	go runRest()
	log.Fatalln(grpcServer.Serve(listener))
}
