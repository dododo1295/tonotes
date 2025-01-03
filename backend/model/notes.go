package model

type Notes struct {
	UserID      string `json:"user_id" bson:"user_id"`
	NoteName    string `bson:"note_name" json:"note_name"`
	NoteContent string `bson:"note_content" json:"note_content"`
}
