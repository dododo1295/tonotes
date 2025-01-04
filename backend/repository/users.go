package repository

import "go.mongodb.org/mongo-driver/mongo"

type UsersRepo struct {
	MongoCollection *mongo.Collection
}
