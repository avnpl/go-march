package utils

import (
	"github.com/joho/godotenv"
	"log"
	"os"
)

func GetEnvVar(key string) string {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalln("Error loading .env file...")
	}
	return os.Getenv(key)
}
