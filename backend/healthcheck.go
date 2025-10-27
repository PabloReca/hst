package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type HealthCheck struct {
	ID           primitive.ObjectID `bson:"_id"`
	Name         string             `bson:"name"`
	URL          string             `bson:"url"`
	Method       string             `bson:"method"`
	Interval     int                `bson:"interval"`
	StatusCode   int                `bson:"statusCode"`
	Headers      map[string]string  `bson:"headers"`
	ExpectedBody *string            `bson:"expectedBody"`
	Status       string             `bson:"status"`
	CreatedAt    time.Time          `bson:"createdAt"`
}

type HealthCheckLog struct {
	Timestamp    time.Time `bson:"timestamp"`
	StatusCode   int       `bson:"statusCode"`
	ResponseTime int64     `bson:"responseTime"`
	Success      bool      `bson:"success"`
	Error        *string   `bson:"error,omitempty"`
}

type HealthCheckCounter struct {
	HealthCheck HealthCheck
	Counter     int
}

type HealthCheckManager struct {
	db          *mongo.Database
	mongoHelper *MongoHelper
	clock       *Clock
	counters    map[string]*HealthCheckCounter
	mu          sync.RWMutex
	client      *http.Client
}

func NewHealthCheckManager(db *mongo.Database, clock *Clock) *HealthCheckManager {
	return &HealthCheckManager{
		db:          db,
		mongoHelper: NewMongoHelper(db),
		clock:       clock,
		counters:    make(map[string]*HealthCheckCounter),
		client:      NewHTTPClientWithTimeout(10 * time.Second),
	}
}

func (m *HealthCheckManager) Start(ctx context.Context) {
	log.Println("Health check manager started")

	m.loadHealthChecks(ctx)

	go m.reloadHealthChecks(ctx)

	tickChan := m.clock.Subscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tickChan:
			m.tick(ctx)
		}
	}
}

func (m *HealthCheckManager) loadHealthChecks(ctx context.Context) {
	var healthChecks []HealthCheck
	err := m.mongoHelper.FindActiveDocuments(ctx, "healthchecks", &healthChecks)
	if err != nil {
		log.Println("Failed to load health checks:", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Create map of active IDs from database
	activeIDs := make(map[string]bool)
	for _, hc := range healthChecks {
		activeIDs[hc.ID.Hex()] = true
	}

	// Remove health checks that are no longer active or were deleted
	for id := range m.counters {
		if !activeIDs[id] {
			log.Printf("Removing health check: %s (deleted or inactive)", m.counters[id].HealthCheck.Name)
			delete(m.counters, id)
		}
	}

	// Add new or update existing health checks
	for _, hc := range healthChecks {
		id := hc.ID.Hex()
		if counter, exists := m.counters[id]; exists {
			// Update if configuration changed
			if counter.HealthCheck.URL != hc.URL ||
				counter.HealthCheck.Interval != hc.Interval ||
				counter.HealthCheck.Method != hc.Method ||
				counter.HealthCheck.StatusCode != hc.StatusCode {
				log.Printf("Updating health check: %s", hc.Name)
				counter.HealthCheck = hc
				counter.Counter = hc.Interval
			}
		} else {
			m.counters[id] = &HealthCheckCounter{
				HealthCheck: hc,
				Counter:     hc.Interval,
			}
			log.Printf("Loaded health check: %s (interval: %ds)", hc.Name, hc.Interval)
		}
	}
}

func (m *HealthCheckManager) reloadHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.loadHealthChecks(ctx)
		}
	}
}

func (m *HealthCheckManager) tick(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, counter := range m.counters {
		counter.Counter--

		if counter.Counter <= 0 {
			go m.executeHealthCheck(ctx, counter.HealthCheck)
			counter.Counter = counter.HealthCheck.Interval
			log.Printf("Executing health check: %s", counter.HealthCheck.Name)
		}
	}
}

func (m *HealthCheckManager) executeHealthCheck(ctx context.Context, hc HealthCheck) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, hc.Method, hc.URL, nil)
	if err != nil {
		m.saveLog(ctx, hc, 0, start, err)
		return
	}

	for key, value := range hc.Headers {
		req.Header.Set(key, value)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		m.saveLog(ctx, hc, 0, start, err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		m.saveLog(ctx, hc, resp.StatusCode, start, err)
		return
	}

	success := resp.StatusCode == hc.StatusCode

	if hc.ExpectedBody != nil && *hc.ExpectedBody != "" {
		expectedBody := strings.TrimSpace(*hc.ExpectedBody)
		actualBody := strings.TrimSpace(string(body))
		if expectedBody != actualBody {
			success = false
		}
	}

	m.saveLog(ctx, hc, resp.StatusCode, start, nil)

	responseTime := time.Since(start).Milliseconds()
	if success {
		log.Printf("[%s] Success - %d in %dms", hc.Name, resp.StatusCode, responseTime)
	} else {
		log.Printf("[%s] Failed - expected %d, got %d in %dms", hc.Name, hc.StatusCode, resp.StatusCode, responseTime)
	}
}

func (m *HealthCheckManager) saveLog(ctx context.Context, hc HealthCheck, statusCode int, start time.Time, err error) {
	responseTime := time.Since(start).Milliseconds()

	logEntry := HealthCheckLog{
		Timestamp:    time.Now(),
		ResponseTime: responseTime,
		StatusCode:   statusCode,
	}

	if err != nil {
		errMsg := err.Error()
		logEntry.Error = &errMsg
		logEntry.Success = false
	} else {
		logEntry.Success = statusCode == hc.StatusCode
	}

	collectionName := fmt.Sprintf("healthcheck_%s", hc.Name)
	
	if err := m.mongoHelper.InsertLog(ctx, collectionName, logEntry); err != nil {
		log.Printf("Failed to save log for %s: %v", hc.Name, err)
	}
}