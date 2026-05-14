package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	httpapi "github.com/itmo-lite-chat/api-gateway-svc/internal/httpapi"
	"github.com/itmo-lite-chat/api-gateway-svc/internal/messages"
	"github.com/itmo-lite-chat/api-gateway-svc/internal/store"
)

type App struct {
	server *http.Server
	client *messages.Client
}

func New() (*App, error) {
	httpAddr := getenv("HTTP_ADDR", ":8080")
	tokenSecret := getenv("TOKEN_SECRET", "lite-chat-dev-secret")
	messagesAddr := getenv("MESSAGES_GRPC_ADDR", "localhost:9999")

	messagesClient, err := messages.NewClient(messagesAddr)
	if err != nil {
		return nil, err
	}

	sessionStore := store.New(tokenSecret)
	handler := httpapi.NewHandler(sessionStore, messagesClient)

	return &App{
		server: &http.Server{
			Addr:              httpAddr,
			Handler:           handler.Routes(),
			ReadHeaderTimeout: 5 * time.Second,
		},
		client: messagesClient,
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
		return a.client.Close()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return a.client.Close()
		}
		_ = a.client.Close()
		return err
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
