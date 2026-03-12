package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	constants "gitlab.com/aparkdev-ing/gopher-runner/constants"
)

var AppConfig Config

func loadEnv() {
	err := godotenv.Load("../.env")
	if err != nil {
		fmt.Println("Warning: .env file not found, using system env")
	}

	AppConfig = Config{
		RegistrationToken: getEnvOrPanic(constants.TOKEN),
		VerifyURL:         getEnvOrPanic(constants.VERIFY_URL),
		RequestURL:        getEnvOrPanic(constants.REQUEST_URL),
		StatusUpdateURL:   getEnvOrPanic(constants.STATUS_UPDATE_URL),
		SendLogURL:        getEnvOrPanic(constants.STATUS_UPDATE_URL),
	}
}

func getEnvOrPanic(key string) string {
	val := os.Getenv(key)
	if val == "" {
		fmt.Printf("CRITICAL: Environment variable %s is not set\n", key)
		os.Exit(1)
	}
	return val
}
