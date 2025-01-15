package utils

import (
	"log"
	"os"
	"strconv"
)

var (
	JWTSecretKey               string
	JWTExpirationTime          int64
	RefreshTokenExpirationTime int64
)

func InitJWT() {

	// For tests, use default values if environment variables aren't set
	if os.Getenv("GO_ENV") == "test" {
		if os.Getenv("JWT_SECRET_KEY") == "" {
			os.Setenv("JWT_SECRET_KEY", "test_secret_key")
		}
		if os.Getenv("JWT_EXPIRATION_TIME") == "" {
			os.Setenv("JWT_EXPIRATION_TIME", "3600")
		}
		if os.Getenv("REFRESH_TOKEN_EXPIRATION_TIME") == "" {
			os.Setenv("REFRESH_TOKEN_EXPIRATION_TIME", "604800")
		}
	}

	// Get values from environment
	JWTSecretKey = os.Getenv("JWT_SECRET_KEY")
	if JWTSecretKey == "" {
		log.Fatal("JWT Secret Key not set")
	}

	jwtExpiration := os.Getenv("JWT_EXPIRATION_TIME")
	if jwtExpiration == "" {
		log.Fatal("JWT Expiration Time not set")
	}

	var err error
	JWTExpirationTime, err = strconv.ParseInt(jwtExpiration, 10, 64)
	if err != nil {
		log.Fatal("Error parsing JWT expiration time")
	}

	refreshToken := os.Getenv("REFRESH_TOKEN_EXPIRATION_TIME")
	if refreshToken == "" {
		log.Fatal("Refresh Token Expiration Time not set")
	}

	RefreshTokenExpirationTime, err = strconv.ParseInt(refreshToken, 10, 64)
	if err != nil {
		log.Fatal("Error parsing refresh token expiration time")
	}
}
