package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/itmo-lite-chat/api-gateway-svc/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	if err := a.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
