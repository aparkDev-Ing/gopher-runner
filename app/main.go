package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	//"runtime"
	//"github.com/joho/godotenv"
)

func main() {
	fmt.Println("Go-Runner Process Starts")
	loadEnv()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	healthCheck(ctx)

	defer stop()

	jobHandler(ctx)
	fmt.Println("[Main] All workers finished. Runner exited gracefully.")
}
