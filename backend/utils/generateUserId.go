package utils

import (
	"log"

	"github.com/google/uuid"
)

func GenerateUserID() string {
	id, err := uuid.NewUUID()
	if err != nil {
		log.Fatal("Failed to generate a unique ID", err)
	}
	return id.String()
}
