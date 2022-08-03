package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type EngWord struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	Text          string             `bson:"text"`
	Hint          string             `bson:"hint"`
	AddedAt       int64              `bson:"added_at"`
	LastTouchedAt int64              `bson:"last_touched_at"`
	TouchedCount  uint               `bson:"touched_count"`
	SuccessCount  uint               `bson:"success_count"`
	FailCount     uint               `bson:"fail_count"`
}

func getEngWordsRepo(mngDB *mongo.Database) *EngWordsRepo {
	return &EngWordsRepo{c: mngDB.Collection("eng_words")}
}

type EngWordsRepo struct {
	c *mongo.Collection
}

func (r *EngWordsRepo) AddNew(ctx context.Context, text string) (string, error) {
	w := EngWord{
		Text:          text,
		AddedAt:       time.Now().Unix(),
		LastTouchedAt: 0,
		TouchedCount:  0,
	}

	res, err := r.c.InsertOne(ctx, &w)
	if err != nil {
		return "", fmt.Errorf("InsertOne: %w", err)
	}

	id := res.InsertedID.(primitive.ObjectID).Hex()

	return id, nil
}

func (r *EngWordsRepo) GetByID(ctx context.Context, id string) (EngWord, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return EngWord{}, fmt.Errorf("primitive.ObjectIDFromHex: %w", err)
	}

	filter := bson.M{"_id": objID}
	res := r.c.FindOne(ctx, filter)
	var w EngWord
	if res.Err() != nil {
		return w, res.Err()
	}

	if err := res.Decode(&w); err != nil {
		return w, err
	}

	return w, nil
}

func (r *EngWordsRepo) PickOneToPractise(ctx context.Context) (EngWord, error) {
	// nolint: govet
	opts := options.FindOne().SetSort(bson.D{{"last_touched_at", 1}})

	res := r.c.FindOne(ctx, bson.D{}, opts)
	var b EngWord
	if res.Err() != nil {
		return b, res.Err()
	}

	if err := res.Decode(&b); err != nil {
		return b, err
	}

	return b, nil
}

func (r *EngWordsRepo) Save(ctx context.Context, w EngWord) error {
	filter := bson.M{"_id": w.ID}
	upd := bson.M{"$set": w}
	upsert := true
	opts := &options.UpdateOptions{Upsert: &upsert}

	_, err := r.c.UpdateOne(ctx, filter, upd, opts)

	return err
}
