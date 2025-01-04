package repository

import (
	"context"
	"log"
	"main/model"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func newMongoClient() *mongo.Client {
	mongoTestClient, err := mongo.Connect(context.Background(),
		options.Client().ApplyURI("mongodb://localhost:27017"))

	if err != nil {
		log.Fatal("error while connecting mongodb", err)
	}

	log.Println("Connected to MongoDB!")

	err = mongoTestClient.Ping(context.Background(), readpref.Primary())

	log.Println("Ping to MongoDB!")

	return mongoTestClient
}

func TestMongoOperations(t *testing.T) {
	mongoTestClient := newMongoClient()
	defer mongoTestClient.Disconnect(context.Background())

	noteUsr1 := uuid.New().String()

	coll := mongoTestClient.Database("tonotes").Collection("testNotes")

	notesRepo := NotesRepo{MongoCollection: coll}

	// insert the note

	t.Run("InsertNotes", func(t *testing.T) {
		user := model.Notes{
			UserID:      noteUsr1,
			CreatedAt:   time.Now(),
			NoteName:    "TESTING NOTES",
			NoteContent: "the quick brown fox jumps over the lazy dog.",
		}
		result, err := notesRepo.InsertNotes(&user)

		if err != nil {
			t.Fatal("insert note failed", err)
		}
		t.Log("inserted note id: ", result)
	})

	t.Run("InsertNotes", func(t *testing.T) {
		user := model.Notes{
			UserID:      noteUsr1,
			CreatedAt:   time.Now(),
			NoteName:    "TESTING NOTES DEUX",
			NoteContent: "the quick brown fox jumps over the lazy dog.",
		}
		result, err := notesRepo.InsertNotes(&user)

		if err != nil {
			t.Fatal("insert note failed", err)
		}
		t.Log("inserted note id: ", result)
	})

	t.Run("InsertNotes", func(t *testing.T) {
		user := model.Notes{
			UserID:      noteUsr1,
			CreatedAt:   time.Now(),
			NoteName:    "TESTING NOTES Three",
			NoteContent: "the quick brown fox jumps over the lazy dog.",
		}
		result, err := notesRepo.InsertNotes(&user)

		if err != nil {
			t.Fatal("insert note failed", err)
		}
		t.Log("inserted note id: ", result)
	})

	t.Run("GetNotesByTitle", func(t *testing.T) {
		note, err := notesRepo.GetNotesByTitle("TESTING NOTES")

		if err != nil {
			t.Fatal("get note failed", err)
		}
		t.Log("note: ", note)
	})

	t.Run("GetAllNotes", func(t *testing.T) {
		notes, err := notesRepo.GetAllNotes()

		if err != nil {
			t.Fatal("get all notes failed", err)
		}
		t.Log("notes: ", notes)
	})

	t.Run("DeleteNotesByName", func(t *testing.T) {
		count, err := notesRepo.DeleteNotesByName("TESTING NOTES Three")

		if err != nil {
			t.Fatal("delete note failed", err)
		}
		t.Log("deleted note count: ", count)
	})

	t.Run("UpdateNotesByTitle", func(t *testing.T) {
		user := model.Notes{
			UserID:      noteUsr1,
			CreatedAt:   time.Now(),
			NoteName:    "TESTING NOTES",
			NoteContent: "TEST SUCCESS.",
		}
		result, err := notesRepo.UpdateNotesByTitle("TESTING NOTES", &user)

		if err != nil {
			t.Fatal("update note failed", err)
		}
		t.Log("updated note count: ", result)
	})
}
