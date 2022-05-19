package main

import (
	"context"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	api2 "gitlab.ozon.dev/VeneLooool/homework-2/api"
	"gitlab.ozon.dev/VeneLooool/homework-2/internal/app/server"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
)

func runRest() {
	/*conn, err := grpc.DialContext(
		context.Background(),
		"0.0.0.0:8080",
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalln("Failed to dial server:", err)
	}

	gwmux := runtime.NewServeMux()
	// Register Greeter
	err = api2.RegisterMailServHandler(context.Background(), gwmux, conn)
	if err != nil {
		log.Fatalln("Failed to register gateway:", err)
	}

	gwServer := &http.Server{
		Addr:    ":8090",
		Handler: gwmux,
	}

	log.Println("Serving gRPC-Gateway on http://0.0.0.0:8090")
	log.Fatalln(gwServer.ListenAndServe())*/
	mux := runtime.NewServeMux()
	err := api2.RegisterMailServHandlerFromEndpoint(context.Background(), mux, "localhost:8080", []grpc.DialOption{grpc.WithInsecure()})
	if err != nil {
		log.Fatal(err)
	}
	server := http.Server{
		Handler: mux,
	}
	l, err := net.Listen("tcp", ":8081")
	if err != nil {
		log.Fatal(err)
	}
	err = server.Serve(l)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	newServer := server.Server{}
	newServer.AddAvailableServices([]string{"imap.gmail.com:993", "imap.yandex.ru:993"})
	fmt.Println("Server has started")
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(server.ValidatorInterceptor),
	}
	grpcServer := grpc.NewServer(opts...)
	api2.RegisterMailServServer(grpcServer, &newServer)
	//err = grpcServer.Serve(listener)
	//if err != nil {
	//	panic(err)
	//}
	//for {
	//	time.Sleep(time.Millisecond * 100)
	//}
	//go func() {
	go runRest()
	log.Fatalln(grpcServer.Serve(listener))
	//}()
	/*
		conn, err := grpc.DialContext(
			context.Background(),
			"0.0.0.0:8080",
			grpc.WithBlock(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Fatalln("Failed to dial server:", err)
		}

		gwmux := runtime.NewServeMux()
		// Register Greeter
		err = api2.RegisterMailServHandler(context.Background(), gwmux, conn)
		if err != nil {
			log.Fatalln("Failed to register gateway:", err)
		}

		gwServer := &http.Server{
			Addr:    ":8090",
			Handler: gwmux,
		}

		log.Println("Serving gRPC-Gateway on http://0.0.0.0:8090")
		log.Fatalln(gwServer.ListenAndServe())*/
}
