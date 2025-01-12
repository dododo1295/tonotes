package utils

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	JWTSecretKey               string
	JWTExpirationTime          int64
	RefreshTokenExpirationTime int64
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env")
	}
	// getting secret key
	JWTSecretKey = os.Getenv("JWT_SECRET_KEY")
	if JWTSecretKey == "" {
		log.Fatalf("Secret Key Read Error")
	}

	// parsing duration into readable code
	jwtExpiration := os.Getenv("JWT_EXPIRATION_TIME")
	if jwtExpiration == "" {
		log.Fatalf("Expiration Time Read Error")
	}

	JWTExpirationTime, err = strconv.ParseInt(jwtExpiration, 10, 64)
	if err != nil {
		log.Fatalf("Error parsing expiration duration")
	}

	// parsing refresh
	refreshToken := os.Getenv("REFRESH_TOKEN_EXPIRATION_TIME")
	if refreshToken == "" {
		log.Fatalf("Refresh Token Read Error")
	}

	RefreshTokenExpirationTime, err = strconv.ParseInt(refreshToken, 10, 64)
	if err != nil {
		log.Fatalf("Error parsing refresh duration")
	}
}
