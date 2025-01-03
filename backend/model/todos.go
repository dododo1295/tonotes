package model

type Todos struct {
	UserID          string `json:"user_id" bson:"user_id"`
	TodoName        string `bson:"todo_name" json:"todo_name"`
	TodoDescription string `bson:"todo_description" json:"todo_description"`
	Complete        bool   `bson:"complete" json:"complete"`
}
