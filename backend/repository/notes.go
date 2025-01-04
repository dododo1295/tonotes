package repository

import (
	"go.mongodb.org/mongo-driver/mongo"
)

type NotesRepo struct {
	MongoCollection *mongo.Collection
}

type Response struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}
