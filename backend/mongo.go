package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoHelper struct {
	db *mongo.Database
}

func NewMongoHelper(db *mongo.Database) *MongoHelper {
	return &MongoHelper{
		db: db,
	}
}

func (h *MongoHelper) InsertLog(ctx context.Context, collectionName string, document interface{}) error {
	collection := h.db.Collection(collectionName)
	
	_, err := collection.InsertOne(ctx, document)
	if err != nil {
		log.Printf("Error saving log to %s: %v", collectionName, err)
		return fmt.Errorf("error inserting into %s: %w", collectionName, err)
	}
	
	return nil
}

func (h *MongoHelper) InsertMetrics(ctx context.Context, collectionName string, metrics interface{}) error {
	collection := h.db.Collection(collectionName)
	
	_, err := collection.InsertOne(ctx, metrics)
	if err != nil {
		log.Printf("Error saving metrics to %s: %v", collectionName, err)
		return fmt.Errorf("error inserting metrics into %s: %w", collectionName, err)
	}
	
	log.Printf("Metrics saved to: %s", collectionName)
	return nil
}

func (h *MongoHelper) FindActiveDocuments(ctx context.Context, collectionName string, results interface{}) error {
	collection := h.db.Collection(collectionName)
	
	cursor, err := collection.Find(ctx, bson.M{"status": "active"})
	if err != nil {
		return fmt.Errorf("error finding active documents in %s: %w", collectionName, err)
	}
	defer cursor.Close(ctx)
	
	if err := cursor.All(ctx, results); err != nil {
		return fmt.Errorf("error decoding documents from %s: %w", collectionName, err)
	}
	
	return nil
}

func (h *MongoHelper) CountDocuments(ctx context.Context, collectionName string, filter bson.M) (int64, error) {
	collection := h.db.Collection(collectionName)
	
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("error counting documents in %s: %w", collectionName, err)
	}
	
	return count, nil
}

func (h *MongoHelper) GetLatestMetrics(ctx context.Context, collectionName string, result interface{}) error {
	collection := h.db.Collection(collectionName)
	
	opts := options.FindOne().SetSort(bson.M{"timestamp": -1})
	
	err := collection.FindOne(ctx, bson.M{}, opts).Decode(result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("no metrics in %s", collectionName)
		}
		return fmt.Errorf("error getting latest metrics from %s: %w", collectionName, err)
	}
	
	return nil
}

func (h *MongoHelper) DeleteOldLogs(ctx context.Context, collectionName string, olderThan time.Duration) (int64, error) {
	collection := h.db.Collection(collectionName)
	
	cutoffTime := time.Now().Add(-olderThan)
	
	result, err := collection.DeleteMany(ctx, bson.M{
		"timestamp": bson.M{"$lt": cutoffTime},
	})
	if err != nil {
		return 0, fmt.Errorf("error deleting old logs from %s: %w", collectionName, err)
	}
	
	if result.DeletedCount > 0 {
		log.Printf("Deleted %d old logs from %s", result.DeletedCount, collectionName)
	}
	
	return result.DeletedCount, nil
}

func (h *MongoHelper) CreateIndexes(ctx context.Context, collectionName string) error {
	collection := h.db.Collection(collectionName)
	
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "timestamp", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "success", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "statusCode", Value: 1}},
		},
	}
	
	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		log.Printf("Error creating indexes in %s: %v", collectionName, err)
		return fmt.Errorf("error creating indexes: %w", err)
	}
	
	log.Printf("Indexes created in: %s", collectionName)
	return nil
}

func (h *MongoHelper) BulkInsertLogs(ctx context.Context, collectionName string, documents []interface{}) error {
	if len(documents) == 0 {
		return nil
	}
	
	collection := h.db.Collection(collectionName)
	
	_, err := collection.InsertMany(ctx, documents)
	if err != nil {
		log.Printf("Error in bulk insert to %s: %v", collectionName, err)
		return fmt.Errorf("error in bulk insert: %w", err)
	}
	
	return nil
}

func (h *MongoHelper) GetCollection(collectionName string) *mongo.Collection {
	return h.db.Collection(collectionName)
}

type LogStats struct {
	TotalRequests      int64   `bson:"totalRequests"`
	SuccessfulRequests int64   `bson:"successfulRequests"`
	FailedRequests     int64   `bson:"failedRequests"`
	AverageTime        float64 `bson:"avgTime"`
	MinTime            int64   `bson:"minTime"`
	MaxTime            int64   `bson:"maxTime"`
}

func (h *MongoHelper) GetLogStats(ctx context.Context, collectionName string) (*LogStats, error) {
	collection := h.db.Collection(collectionName)
	
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":                nil,
				"totalRequests":      bson.M{"$sum": 1},
				"successfulRequests": bson.M{"$sum": bson.M{"$cond": []interface{}{"$success", 1, 0}}},
				"failedRequests":     bson.M{"$sum": bson.M{"$cond": []interface{}{"$success", 0, 1}}},
				"avgTime":            bson.M{"$avg": "$responseTime"},
				"minTime":            bson.M{"$min": "$responseTime"},
				"maxTime":            bson.M{"$max": "$responseTime"},
			},
		},
	}
	
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("error in aggregation: %w", err)
	}
	defer cursor.Close(ctx)
	
	var results []LogStats
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("error decoding results: %w", err)
	}
	
	if len(results) == 0 {
		return &LogStats{}, nil
	}
	
	return &results[0], nil
}

func (h *MongoHelper) CollectionExists(ctx context.Context, collectionName string) (bool, error) {
	collections, err := h.db.ListCollectionNames(ctx, bson.M{"name": collectionName})
	if err != nil {
		return false, fmt.Errorf("error listing collections: %w", err)
	}
	
	return len(collections) > 0, nil
}

func (h *MongoHelper) LoadTestNameExists(ctx context.Context, testName string) (bool, error) {
	collection := h.db.Collection("loadtesting_metrics")
	
	count, err := collection.CountDocuments(ctx, bson.M{"name": testName})
	if err != nil {
		return false, fmt.Errorf("error checking test name: %w", err)
	}
	
	return count > 0, nil
}

func (h *MongoHelper) DropCollection(ctx context.Context, collectionName string) error {
	collection := h.db.Collection(collectionName)
	
	err := collection.Drop(ctx)
	if err != nil {
		return fmt.Errorf("error dropping collection %s: %w", collectionName, err)
	}
	
	log.Printf("Collection dropped: %s", collectionName)
	return nil
}