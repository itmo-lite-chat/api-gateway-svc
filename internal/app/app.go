package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/itmo-lite-chat/api-gateway-svc/internal/chats"
	httpapi "github.com/itmo-lite-chat/api-gateway-svc/internal/httpapi"
	"github.com/itmo-lite-chat/api-gateway-svc/internal/messages"
	"github.com/itmo-lite-chat/api-gateway-svc/internal/users"
)

type App struct {
	server         *http.Server
	messagesClient *messages.Client
	usersClient    *users.Client
	chatsClient    *chats.Client
}

func New() (*App, error) {
	httpAddr := getenv("HTTP_ADDR", ":8080")
	messagesAddr := getenv("MESSAGES_GRPC_ADDR", "localhost:9990")
	usersAddr := getenv("USERS_GRPC_ADDR", "localhost:9991")
	chatsAddr := getenv("CHATS_GRPC_ADDR", "localhost:9992")

	messagesClient, err := messages.NewClient(messagesAddr)
	if err != nil {
		return nil, err
	}

	usersClient, err := users.NewClient(usersAddr)
	if err != nil {
		_ = messagesClient.Close()
		return nil, err
	}

	chatsClient, err := chats.NewClient(chatsAddr)
	if err != nil {
		_ = messagesClient.Close()
		_ = usersClient.Close()
		return nil, err
	}

	handler := httpapi.NewHandler(messagesClient, usersClient, chatsClient)

	return &App{
		server: &http.Server{
			Addr:              httpAddr,
			Handler:           handler.Routes(),
			ReadHeaderTimeout: 5 * time.Second,
		},
		messagesClient: messagesClient,
		usersClient:    usersClient,
		chatsClient:    chatsClient,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.server.Shutdown(shutdownCtx)
		return a.close()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return a.close()
		}
		_ = a.close()
		return err
	}
}

func (a *App) close() error {
	if err := a.messagesClient.Close(); err != nil {
		_ = a.usersClient.Close()
		_ = a.chatsClient.Close()
		return err
	}
	if err := a.usersClient.Close(); err != nil {
		_ = a.chatsClient.Close()
		return err
	}
	return a.chatsClient.Close()
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
