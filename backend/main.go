package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	ctx := context.Background()

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://admin:password123@localhost:27017"
	}

	mongoDatabase := os.Getenv("MONGO_DATABASE")
	if mongoDatabase == "" {
		mongoDatabase = "hts-config"
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(mongoDatabase)
	log.Println("Connected to MongoDB")

	clock := NewClock()
	healthCheckManager := NewHealthCheckManager(db, clock)
	loadTestServer := NewLoadTestServer("8080", db)

	go clock.Start(ctx)
	go healthCheckManager.Start(ctx)
	go func() {
		if err := loadTestServer.Start(ctx); err != nil && err != http.ErrServerClosed {
			log.Printf("Error in load test server: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down")
	clock.Stop()
}