package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	ModeNewWord    = 1
	ModeHint       = 2
	ModePractising = 3
)

const statusID = "mode_id"

type Status struct {
	ID           string `bson:"_id,omitempty"`
	Mode         int    `bson:"mode"`
	WordID       string `bson:"word_id"`
	BtnMessageID int    `bson:"btn_message_id"`
}

func getStatusRepo(mngDB *mongo.Database) *StatusRepo {
	return &StatusRepo{c: mngDB.Collection("eng_words_status")}
}

type StatusRepo struct {
	c *mongo.Collection
}

func (r *StatusRepo) Get(ctx context.Context, id string) (Status, error) {
	filter := bson.M{"_id": statusID}
	res := r.c.FindOne(ctx, filter)
	if res.Err() == mongo.ErrNoDocuments {
		return Status{
			ID:   id,
			Mode: ModeNewWord,
		}, nil
	}
	var s Status
	if res.Err() != nil {
		return s, res.Err()
	}

	if err := res.Decode(&s); err != nil {
		return s, err
	}

	return s, nil
}

func (r *StatusRepo) Save(ctx context.Context, s Status) error {
	s.ID = statusID
	filter := bson.M{"_id": statusID}
	upd := bson.M{"$set": s}
	upsert := true
	opts := &options.UpdateOptions{Upsert: &upsert}

	_, err := r.c.UpdateOne(ctx, filter, upd, opts)

	return err
}
