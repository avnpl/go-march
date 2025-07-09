package utils

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"

	utilErrs "github.com/avnpl/go-march/utils/errors"
)

func GetEnvVar(key string) string {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalln("Error loading .env file...")
	}
	return os.Getenv(key)
}

func GetRequestBodyAsString(byteArr []byte) (string, error) {
	if len(byteArr) >= 32 {
		return "", fmt.Errorf("request body is too long : %w", utilErrs.ErrInternal)
	}
	return string(byteArr), nil
}
