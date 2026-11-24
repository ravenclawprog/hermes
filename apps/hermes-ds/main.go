package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hermes-ds/internal/api"
	"hermes-ds/internal/app"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	service, err := app.NewDemo(os.Getenv("HERMES_DS_DEMO_PASSWORD"))
	if err != nil {
		return err
	}

	port := os.Getenv("HERMES_DS_PORT")
	if port == "" {
		port = "8080"
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		return err
	}
	defer listener.Close()

	server := &http.Server{
		Handler: api.LogRequests(api.New(service)),
		ConnState: func(conn net.Conn, state http.ConnState) {
			if state == http.StateNew || state == http.StateClosed {
				log.Printf("connection %s %s", conn.RemoteAddr(), state)
			}
		},
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("hermes-ds demo listening at http://%s", listener.Addr())
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			return err
		}
		if err := <-result; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
