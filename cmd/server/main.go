package main

import (
	"context"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	api2 "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"gitlab.ozon.dev/VeneLooool/homework-2/config"
	"gitlab.ozon.dev/VeneLooool/homework-2/internal/app/server"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
)

func main() {
	conf, err := config.GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	serv := server.NewServer(conf)

	listen, err := net.Listen(conf.GetNetwork(), conf.GetAddressPort())
	if err != nil {
		panic(err)
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(server.Interceptor),
	}
	grpcServer := grpc.NewServer(opts...)
	api2.RegisterMailServServer(grpcServer, serv)

	go runRest(conf)

	log.Fatal(grpcServer.Serve(listen))
}

func runRest(config config.Config) {
	mux := runtime.NewServeMux()
	err := api2.RegisterMailServHandlerFromEndpoint(
		context.Background(),
		mux,
		config.GetAddressPort(),
		[]grpc.DialOption{grpc.WithInsecure()},
	)

	if err != nil {
		log.Fatal(err)
	}

	server := http.Server{
		Handler: mux,
	}
	listen, err := net.Listen(config.GetHttpNetwork(), config.GetHttpAddress())
	if err != nil {
		log.Fatal(err)
	}
	if err = server.Serve(listen); err != nil {
		log.Fatal(err)
	}
}
