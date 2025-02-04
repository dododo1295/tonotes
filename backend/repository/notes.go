package repository

import (
	"context"
	"errors"
	"fmt"
	"main/model"
	"main/utils"
	"os"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type NotesRepo struct {
	MongoCollection *mongo.Collection
}

type SearchOptions struct {
	UserID      string
	Query       string
	Tags        []string
	MatchAll    bool
	Page        int
	PageSize    int
	SortBy      string
	SortOrder   string
	SearchScore bool
}

func GetNotesRepo(client *mongo.Client) *NotesRepo {
	dbName := os.Getenv("MONGO_DB")
	collectionName := os.Getenv("NOTES_COLLECTION")

	if os.Getenv("GO_ENV") == "test" {
		dbName = "tonotes_test"
	}
	return &NotesRepo{
		MongoCollection: client.Database(dbName).Collection(collectionName),
	}
}

// CountUserNotes counts the number of notes for a user
func (r *NotesRepo) CountUserNotes(userID string) (int, error) {
	count, err := r.MongoCollection.CountDocuments(context.Background(),
		bson.M{"user_id": userID})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// GetAllTags retrieves all unique tags used by a user
func (r *NotesRepo) GetAllTags(userID string) ([]string, error) {
	// Using MongoDB's distinct command to get unique tags
	tags, err := r.MongoCollection.Distinct(
		context.Background(),
		"tags",
		bson.M{"user_id": userID},
	)
	if err != nil {
		return nil, err
	}

	// Convert interface{} array to string array
	stringTags := make([]string, 0)
	for _, tag := range tags {
		if strTag, ok := tag.(string); ok {
			stringTags = append(stringTags, strTag)
		}
	}

	return stringTags, nil
}

// CountNotesByTag counts notes for each tag
func (r *NotesRepo) CountNotesByTag(userID string) (map[string]int, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"user_id":     userID,
				"is_archived": false,
			},
		},
		{
			"$unwind": "$tags",
		},
		{
			"$group": bson.M{
				"_id":   "$tags",
				"count": bson.M{"$sum": 1},
			},
		},
	}

	cursor, err := r.MongoCollection.Aggregate(context.Background(), pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	type tagCount struct {
		ID    string `bson:"_id"`
		Count int    `bson:"count"`
	}

	var results []tagCount
	if err = cursor.All(context.Background(), &results); err != nil {
		return nil, err
	}

	tagCounts := make(map[string]int)
	for _, result := range results {
		tagCounts[result.ID] = result.Count
	}

	return tagCounts, nil
}

// CreateNote creates a new note
func (r *NotesRepo) CreateNote(note *model.Notes) error {
	timer := utils.TrackDBOperation("insert", "notes")
	defer timer.ObserveDuration()

	if note.UserID == "" {
		utils.TrackError("database", "missing_user_id")
		return errors.New("user ID is required")
	}

	note.CreatedAt = time.Now()
	note.UpdatedAt = time.Now()

	_, err := r.MongoCollection.InsertOne(context.Background(), note)
	if err != nil {
		utils.TrackError("database", "note_creation_failed")
		return err
	}

	utils.TrackNoteCreation(note.UserID)
	return nil
}

// GetUserNotes retrieves all notes for a user
func (r *NotesRepo) GetUserNotes(userID string) ([]*model.Notes, error) {
	var notes []*model.Notes
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.MongoCollection.Find(context.Background(),
		bson.M{"user_id": userID, "is_archived": false}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// GetNote retrieves a specific note
func (r *NotesRepo) GetNote(noteID string, userID string) (*model.Notes, error) {
	var note model.Notes
	err := r.MongoCollection.FindOne(context.Background(),
		bson.M{"_id": noteID, "user_id": userID}).Decode(&note)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("note not found")
		}
		return nil, err
	}
	return &note, nil
}

// UpdateNote updates a specific note
func (r *NotesRepo) UpdateNote(noteID string, userID string, updates *model.Notes) error {
	timer := utils.TrackDBOperation("update", "notes")
	defer timer.ObserveDuration()

	updates.UpdatedAt = time.Now()

	filter := bson.M{
		"_id":     noteID,
		"user_id": userID,
	}

	update := bson.M{
		"$set": bson.M{
			"title":      updates.Title,
			"content":    updates.Content,
			"tags":       updates.Tags,
			"updated_at": updates.UpdatedAt,
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		utils.TrackError("database", "note_update_failed")
		return err
	}

	if result.MatchedCount == 0 {
		utils.TrackError("database", "note_not_found")
		return errors.New("note not found")
	}

	return nil
}

// DeleteNote deletes a specific note
func (r *NotesRepo) DeleteNote(noteID string, userID string) error {
	timer := utils.TrackDBOperation("delete", "notes")
	defer timer.ObserveDuration()

	filter := bson.M{
		"_id":     noteID,
		"user_id": userID,
	}

	result, err := r.MongoCollection.DeleteOne(context.Background(), filter)
	if err != nil {
		utils.TrackError("database", "note_deletion_failed")
		return err
	}

	if result.DeletedCount == 0 {
		utils.TrackError("database", "note_not_found")
		return errors.New("note not found")
	}

	return nil
}

// ArchiveNote toggles the archived status of a note
func (r *NotesRepo) ArchiveNote(noteID string, userID string) error {
	timer := utils.TrackDBOperation("update", "notes")
	defer timer.ObserveDuration()

	var note model.Notes
	filter := bson.M{
		"_id":     noteID,
		"user_id": userID,
	}

	err := r.MongoCollection.FindOne(context.Background(), filter).Decode(&note)
	if err != nil {
		utils.TrackError("database", "note_not_found")
		return err
	}

	update := bson.M{
		"$set": bson.M{
			"is_archived": !note.IsArchived,
			"updated_at":  time.Now(),
		},
	}

	result, err := r.MongoCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		utils.TrackError("database", "archive_operation_failed")
		return err
	}

	if result.MatchedCount == 0 {
		utils.TrackError("database", "note_not_found")
		return errors.New("note not found")
	}

	return nil
}

// GetArchivedNotes retrieves all archived notes for a user
func (r *NotesRepo) GetArchivedNotes(userID string) ([]*model.Notes, error) {
	var notes []*model.Notes
	opts := options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}})

	cursor, err := r.MongoCollection.Find(context.Background(),
		bson.M{"user_id": userID, "is_archived": true}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// Pin-related operations
func (r *NotesRepo) pinNote(ctx context.Context, noteID string, userID string) error {
	// Get current count of pinned notes
	count, err := r.MongoCollection.CountDocuments(ctx, bson.M{
		"user_id":   userID,
		"is_pinned": true,
	})
	if err != nil {
		return err
	}

	// Update the note with pin and position
	result, err := r.MongoCollection.UpdateOne(ctx,
		bson.M{"_id": noteID, "user_id": userID},
		bson.M{
			"$set": bson.M{
				"is_pinned":       true,
				"pinned_position": count + 1,
				"updated_at":      time.Now(),
			},
		})
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return errors.New("note not found")
	}

	return nil
}

func (r *NotesRepo) unpinNote(ctx context.Context, noteID string, userID string) error {
	// Get the current note to find its position
	var note model.Notes
	err := r.MongoCollection.FindOne(ctx,
		bson.M{"_id": noteID, "user_id": userID}).Decode(&note)
	if err != nil {
		return err
	}

	// Remove pin and position
	_, err = r.MongoCollection.UpdateOne(ctx,
		bson.M{"_id": noteID},
		bson.M{
			"$set": bson.M{
				"is_pinned":       false,
				"pinned_position": nil,
				"updated_at":      time.Now(),
			},
		})
	if err != nil {
		return err
	}

	// Update positions of remaining pinned notes
	_, err = r.MongoCollection.UpdateMany(ctx,
		bson.M{
			"user_id":         userID,
			"is_pinned":       true,
			"pinned_position": bson.M{"$gt": note.PinnedPosition},
		},
		bson.M{
			"$inc": bson.M{"pinned_position": -1},
		})
	return err
}

// TogglePin now handles pinning and unpinning with positions
func (r *NotesRepo) TogglePin(noteID string, userID string) error {
	session, err := r.MongoCollection.Database().Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(context.Background())

	return mongo.WithSession(context.Background(), session, func(sc mongo.SessionContext) error {
		var note model.Notes
		filter := bson.M{
			"_id":     noteID,
			"user_id": userID,
		}

		err := r.MongoCollection.FindOne(sc, filter).Decode(&note)
		if err != nil {
			return err
		}

		if note.IsPinned {
			return r.unpinNote(sc, noteID, userID)
		} else {
			return r.pinNote(sc, noteID, userID)
		}
	})
}

// UpdatePinPosition updates the position of a pinned note
func (r *NotesRepo) UpdatePinPosition(noteID string, userID string, newPosition int) error {
	// Get total pinned notes first to validate position
	pinnedNotes, err := r.GetPinnedNotes(userID)
	if err != nil {
		return err
	}

	// Validate new position
	if newPosition < 1 || newPosition > len(pinnedNotes) {
		return fmt.Errorf("invalid position")
	}

	session, err := r.MongoCollection.Database().Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(context.Background())

	return mongo.WithSession(context.Background(), session, func(sc mongo.SessionContext) error {
		var note model.Notes
		err := r.MongoCollection.FindOne(sc,
			bson.M{"_id": noteID, "user_id": userID}).Decode(&note)
		if err != nil {
			return err
		}

		if !note.IsPinned {
			return errors.New("note is not pinned")
		}

		currentPos := note.PinnedPosition

		// Update positions of notes between old and new position
		var updateQuery bson.M
		if newPosition > currentPos {
			updateQuery = bson.M{
				"user_id":   userID,
				"is_pinned": true,
				"pinned_position": bson.M{
					"$gt":  currentPos,
					"$lte": newPosition,
				},
			}
		} else {
			updateQuery = bson.M{
				"user_id":   userID,
				"is_pinned": true,
				"pinned_position": bson.M{
					"$gte": newPosition,
					"$lt":  currentPos,
				},
			}
		}

		// Shift other notes' positions
		_, err = r.MongoCollection.UpdateMany(sc,
			updateQuery,
			bson.M{
				"$inc": bson.M{
					"pinned_position": map[bool]int{true: 1, false: -1}[newPosition < currentPos],
				},
			})
		if err != nil {
			return err
		}

		// Update target note's position
		_, err = r.MongoCollection.UpdateOne(sc,
			bson.M{"_id": noteID},
			bson.M{
				"$set": bson.M{
					"pinned_position": newPosition,
					"updated_at":      time.Now(),
				},
			})
		return err
	})
}

// GetPinnedNotes retrieves all pinned notes for a user
func (r *NotesRepo) GetPinnedNotes(userID string) ([]*model.Notes, error) {
	var notes []*model.Notes
	opts := options.Find().SetSort(bson.D{{Key: "pinned_position", Value: 1}})

	cursor, err := r.MongoCollection.Find(context.Background(),
		bson.M{
			"user_id":     userID,
			"is_pinned":   true,
			"is_archived": false,
		}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *NotesRepo) GetSearchSuggestions(userID, prefix string) ([]string, error) {
	if strings.TrimSpace(prefix) == "" {
		return []string{}, nil
	}

	prefix = strings.ToLower(strings.TrimSpace(prefix))

	pipeline := []bson.M{
		{
			"$match": bson.M{"user_id": userID},
		},
		{
			"$project": bson.M{
				"words": bson.M{
					"$concatArrays": []interface{}{
						bson.M{"$split": []interface{}{"$title", " "}},
						"$tags",
					},
				},
			},
		},
		{
			"$unwind": "$words",
		},
		{
			"$project": bson.M{
				"word": bson.M{"$toLower": "$words"},
			},
		},
		{
			"$match": bson.M{
				"word": bson.M{
					"$regex": primitive.Regex{
						Pattern: "^" + regexp.QuoteMeta(prefix),
						Options: "i",
					},
				},
			},
		},
		{
			"$group": bson.M{"_id": "$word"},
		},
		{
			"$sort": bson.M{"_id": 1},
		},
	}

	cursor, err := r.MongoCollection.Aggregate(context.Background(), pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var results []struct {
		ID string `bson:"_id"`
	}
	if err = cursor.All(context.Background(), &results); err != nil {
		return nil, err
	}

	suggestions := make([]string, 0, len(results))
	for _, result := range results {
		if word := strings.TrimSpace(result.ID); word != "" {
			suggestions = append(suggestions, word)
		}
	}

	return suggestions, nil
}

func (r *NotesRepo) FindNotes(ctx context.Context, opts SearchOptions) ([]*model.Notes, error) {
	filter := bson.M{"user_id": opts.UserID}

	// Add text search if query provided
	if opts.Query != "" {
		if strings.Contains(opts.Query, " ") {
			// For multi-word queries, use $or with $regex
			words := strings.Fields(opts.Query)
			regexQueries := make([]bson.M, 0)
			for _, word := range words {
				regex := primitive.Regex{Pattern: word, Options: "i"}
				regexQueries = append(regexQueries,
					bson.M{"title": bson.M{"$regex": regex}},
					bson.M{"content": bson.M{"$regex": regex}},
				)
			}
			filter["$or"] = regexQueries
		} else {
			// For single-word queries, use text search
			filter["$text"] = bson.M{"$search": opts.Query}
		}
	}

	// Add tags filter if provided
	if len(opts.Tags) > 0 {
		if opts.MatchAll {
			filter["tags"] = bson.M{"$all": opts.Tags}
		} else {
			filter["tags"] = bson.M{"$in": opts.Tags}
		}
	}

	findOptions := options.Find()

	// Configure sorting
	if opts.SortBy != "" {
		sortOrder := -1
		if opts.SortOrder == "asc" {
			sortOrder = 1
		}
		findOptions.SetSort(bson.M{opts.SortBy: sortOrder})
	} else {
		findOptions.SetSort(bson.M{"created_at": -1})
	}

	// Execute query
	cursor, err := r.MongoCollection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to execute find query: %w", err)
	}
	defer cursor.Close(ctx)

	var notes []*model.Notes
	if err := cursor.All(ctx, &notes); err != nil {
		return nil, fmt.Errorf("failed to decode notes: %w", err)
	}

	return notes, nil
}

// helper functions

func countMatches(note *model.Notes, query string) int {
	query = strings.ToLower(query)
	terms := strings.Fields(query)
	matches := 0

	// Check title (weighted higher)
	title := strings.ToLower(note.Title)
	for _, term := range terms {
		if strings.Contains(title, term) {
			matches += 2 // Title matches count double
		}
	}

	// Check content
	content := strings.ToLower(note.Content)
	for _, term := range terms {
		if strings.Contains(content, term) {
			matches++
		}
	}

	// Check tags
	for _, tag := range note.Tags {
		tag = strings.ToLower(tag)
		for _, term := range terms {
			if strings.Contains(tag, term) {
				matches++
			}
		}
	}

	return matches
}
