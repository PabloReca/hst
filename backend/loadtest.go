package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

type LoadTestRequest struct {
	Name               string            `json:"name"`
	URL                string            `json:"url"`
	Method             string            `json:"method"`
	Headers            map[string]string `json:"headers"`
	Body               string            `json:"body"`
	CallsPerThread     int               `json:"callsPerThread"`
	Threads            int               `json:"threads"`
	Timeout            int               `json:"timeout"` // seconds
	ExpectedStatusCode int               `json:"expectedStatusCode,omitempty"`
}

type LoadTestResult struct {
	Name               string         `bson:"name" json:"name"`
	TestConfig         LoadTestConfig `bson:"testConfig" json:"testConfig"`
	TotalRequests      int            `bson:"totalRequests" json:"totalRequests"`
	SuccessfulRequests int            `bson:"successfulRequests" json:"successfulRequests"`
	FailedRequests     int            `bson:"failedRequests" json:"failedRequests"`
	TotalDuration      float64        `bson:"totalDuration" json:"totalDuration"` // seconds
	RequestsPerSecond  float64        `bson:"requestsPerSecond" json:"requestsPerSecond"`
	AverageTime        float64        `bson:"averageTime" json:"averageTime"` // ms
	MinTime            float64        `bson:"minTime" json:"minTime"`         // ms
	MaxTime            float64        `bson:"maxTime" json:"maxTime"`         // ms
	MedianTime         float64        `bson:"medianTime" json:"medianTime"`   // ms
	P95Time            float64        `bson:"p95Time" json:"p95Time"`         // ms
	P99Time            float64        `bson:"p99Time" json:"p99Time"`         // ms
	StatusCodes        map[int]int    `bson:"statusCodes" json:"statusCodes"`
	ErrorCount         int            `bson:"errorCount" json:"errorCount"`
	TotalBytesReceived int64          `bson:"totalBytesReceived" json:"totalBytesReceived"`
	ThroughputMBps     float64        `bson:"throughputMBps" json:"throughputMBps"`
	SuccessRate        float64        `bson:"successRate" json:"successRate"`
	Timestamp          time.Time      `bson:"timestamp" json:"timestamp"`
}

type LoadTestConfig struct {
	URL                string            `bson:"url" json:"url"`
	Method             string            `bson:"method" json:"method"`
	Headers            map[string]string `bson:"headers,omitempty" json:"headers,omitempty"`
	Body               string            `bson:"body,omitempty" json:"body,omitempty"`
	CallsPerThread     int               `bson:"callsPerThread" json:"callsPerThread"`
	Threads            int               `bson:"threads" json:"threads"`
	TotalCalls         int               `bson:"totalCalls" json:"totalCalls"`
	Timeout            int               `bson:"timeout" json:"timeout"`
	ExpectedStatusCode int               `bson:"expectedStatusCode" json:"expectedStatusCode"`
}

type LoadTestLog struct {
	Name         string    `bson:"name"`
	URL          string    `bson:"url"`
	Method       string    `bson:"method"`
	StatusCode   int       `bson:"statusCode"`
	ResponseTime float64   `bson:"responseTime"` // ms
	Success      bool      `bson:"success"`
	Error        *string   `bson:"error,omitempty"`
	Timestamp    time.Time `bson:"timestamp"`
}

type RequestResult struct {
	StatusCode    int
	ResponseTime  time.Duration
	BytesReceived int64
	Error         error
}

type LoadTestExecutor struct {
	client      *http.Client
	db          *mongo.Database
	mongoHelper *MongoHelper
}

func NewLoadTestExecutor(timeout time.Duration, db *mongo.Database) *LoadTestExecutor {
	return &LoadTestExecutor{
		client: &http.Client{
			Timeout: timeout,
		},
		db:          db,
		mongoHelper: NewMongoHelper(db),
	}
}

func (e *LoadTestExecutor) Execute(ctx context.Context, req LoadTestRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	
	exists, err := e.mongoHelper.LoadTestNameExists(ctx, req.Name)
	if err != nil {
		return fmt.Errorf("error checking test name: %v", err)
	}
	if exists {
		return fmt.Errorf("load test with name '%s' already exists. Please use a different name", req.Name)
	}
	
	if req.CallsPerThread <= 0 {
		return fmt.Errorf("callsPerThread must be greater than 0")
	}
	if req.Threads <= 0 {
		return fmt.Errorf("threads must be greater than 0")
	}
	if req.Method == "" {
		req.Method = "GET"
	}
	if req.ExpectedStatusCode == 0 {
		req.ExpectedStatusCode = 200
	}
	if req.Timeout > 0 {
		e.client.Timeout = time.Duration(req.Timeout) * time.Second
	}

	totalCalls := req.CallsPerThread * req.Threads

	log.Printf("Starting load test '%s': %d threads x %d calls = %d total requests to %s", 
		req.Name, req.Threads, req.CallsPerThread, totalCalls, req.URL)

	startTime := time.Now()
	
	results := make(chan RequestResult, totalCalls)
	var wg sync.WaitGroup
	
	jobs := make(chan int, totalCalls)
	
	// Launch worker threads
	for i := 0; i < req.Threads; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for range jobs {
				result := e.executeRequest(ctx, req)
				results <- result
				e.saveLog(ctx, req, result)
			}
		}(i)
	}
	
	// Send jobs to workers
	go func() {
		for i := 0; i < totalCalls; i++ {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- i:
			}
		}
		close(jobs)
	}()
	
	wg.Wait()
	close(results)
	
	totalDuration := time.Since(startTime)
	
	return e.processAndSaveResults(ctx, req, results, totalDuration, totalCalls)
}

func (e *LoadTestExecutor) executeRequest(ctx context.Context, testReq LoadTestRequest) RequestResult {
	start := time.Now()
	
	var bodyReader io.Reader
	if testReq.Body != "" {
		bodyReader = bytes.NewBufferString(testReq.Body)
	}
	
	req, err := http.NewRequestWithContext(ctx, testReq.Method, testReq.URL, bodyReader)
	if err != nil {
		return RequestResult{
			Error:        err,
			ResponseTime: time.Since(start),
		}
	}
	
	for key, value := range testReq.Headers {
		req.Header.Set(key, value)
	}
	
	resp, err := e.client.Do(req)
	responseTime := time.Since(start)
	
	if err != nil {
		return RequestResult{
			Error:        err,
			ResponseTime: responseTime,
		}
	}
	defer resp.Body.Close()
	
	bytesReceived, _ := io.Copy(io.Discard, resp.Body)
	
	return RequestResult{
		StatusCode:    resp.StatusCode,
		ResponseTime:  responseTime,
		BytesReceived: bytesReceived,
	}
}

func (e *LoadTestExecutor) saveLog(ctx context.Context, req LoadTestRequest, result RequestResult) {
	success := result.Error == nil && result.StatusCode == req.ExpectedStatusCode
	
	logEntry := LoadTestLog{
		Name:         req.Name,
		URL:          req.URL,
		Method:       req.Method,
		StatusCode:   result.StatusCode,
		ResponseTime: float64(result.ResponseTime.Milliseconds()),
		Success:      success,
		Timestamp:    time.Now(),
	}
	
	if result.Error != nil {
		errMsg := result.Error.Error()
		logEntry.Error = &errMsg
	}
	
	collectionName := fmt.Sprintf("loadtest_logs_%s", req.Name)
	
	if err := e.mongoHelper.InsertLog(ctx, collectionName, logEntry); err != nil {
		// Error already logged in MongoHelper
	}
}

func (e *LoadTestExecutor) processAndSaveResults(ctx context.Context, req LoadTestRequest, results chan RequestResult, totalDuration time.Duration, totalCalls int) error {
	var (
		totalRequests      int
		successfulRequests int
		failedRequests     int
		totalTime          int64
		totalBytes         int64
		responseTimes      []float64
		statusCodes        = make(map[int]int)
		errorCount         int
		minTime            = float64(^uint64(0) >> 1) // Max float64
		maxTime            float64
	)
	
	for result := range results {
		totalRequests++
		responseTimeMs := float64(result.ResponseTime.Milliseconds())
		responseTimes = append(responseTimes, responseTimeMs)
		totalTime += result.ResponseTime.Milliseconds()
		totalBytes += result.BytesReceived
		
		if responseTimeMs < minTime {
			minTime = responseTimeMs
		}
		if responseTimeMs > maxTime {
			maxTime = responseTimeMs
		}
		
		if result.Error != nil {
			failedRequests++
			errorCount++
		} else {
			if result.StatusCode == req.ExpectedStatusCode {
				successfulRequests++
			} else {
				failedRequests++
			}
			statusCodes[result.StatusCode]++
		}
	}
	
	if totalRequests == 0 {
		return fmt.Errorf("no requests were executed")
	}
	
	avgTime := float64(totalTime) / float64(totalRequests)
	rps := float64(totalRequests) / totalDuration.Seconds()
	successRate := (float64(successfulRequests) / float64(totalRequests)) * 100
	throughputMBps := (float64(totalBytes) / 1024 / 1024) / totalDuration.Seconds()
	
	sort.Float64s(responseTimes)
	
	median := calculatePercentile(responseTimes, 50)
	p95 := calculatePercentile(responseTimes, 95)
	p99 := calculatePercentile(responseTimes, 99)
	
	if minTime == float64(^uint64(0)>>1) {
		minTime = 0
	}
	
	result := LoadTestResult{
		Name: req.Name,
		TestConfig: LoadTestConfig{
			URL:                req.URL,
			Method:             req.Method,
			Headers:            req.Headers,
			Body:               req.Body,
			CallsPerThread:     req.CallsPerThread,
			Threads:            req.Threads,
			TotalCalls:         totalCalls,
			Timeout:            req.Timeout,
			ExpectedStatusCode: req.ExpectedStatusCode,
		},
		TotalRequests:      totalRequests,
		SuccessfulRequests: successfulRequests,
		FailedRequests:     failedRequests,
		TotalDuration:      totalDuration.Seconds(),
		RequestsPerSecond:  rps,
		AverageTime:        avgTime,
		MinTime:            minTime,
		MaxTime:            maxTime,
		MedianTime:         median,
		P95Time:            p95,
		P99Time:            p99,
		StatusCodes:        statusCodes,
		ErrorCount:         errorCount,
		TotalBytesReceived: totalBytes,
		ThroughputMBps:     throughputMBps,
		SuccessRate:        successRate,
		Timestamp:          time.Now(),
	}
	
	collectionName := "loadtest_metrics"
	
	if err := e.mongoHelper.InsertMetrics(ctx, collectionName, result); err != nil {
		return fmt.Errorf("error saving metrics: %v", err)
	}
	
	log.Printf("Load test '%s' completed: %d/%d successful (%.1f%%), %.2f req/s, avg: %.2fms, throughput: %.2f MB/s", 
		req.Name, result.SuccessfulRequests, result.TotalRequests, result.SuccessRate, 
		result.RequestsPerSecond, result.AverageTime, result.ThroughputMBps)
	
	return nil
}

func calculatePercentile(sortedTimes []float64, percentile float64) float64 {
	if len(sortedTimes) == 0 {
		return 0
	}
	
	index := int(float64(len(sortedTimes)) * percentile / 100.0)
	if index >= len(sortedTimes) {
		index = len(sortedTimes) - 1
	}
	
	return sortedTimes[index]
}

type LoadTestServer struct {
	executor *LoadTestExecutor
	port     string
	db       *mongo.Database
}

func NewLoadTestServer(port string, db *mongo.Database) *LoadTestServer {
	return &LoadTestServer{
		executor: NewLoadTestExecutor(30*time.Second, db),
		port:     port,
		db:       db,
	}
}

func (s *LoadTestServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/loadtest", s.handleLoadTest)
	mux.HandleFunc("/health", s.handleHealth)
	
	server := &http.Server{
		Addr:    ":" + s.port,
		Handler: s.loggingMiddleware(mux),
	}
	
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()
	
	log.Printf("Load test server listening on port %s", s.port)
	return server.ListenAndServe()
}

func (s *LoadTestServer) handleLoadTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req LoadTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding JSON: %v", err), http.StatusBadRequest)
		return
	}
	
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	
	ctx := context.Background()
	mongoHelper := NewMongoHelper(s.db)
	exists, err := mongoHelper.LoadTestNameExists(ctx, req.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error checking name: %v", err), http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, fmt.Sprintf("Load test with name '%s' already exists. Please use a different name", req.Name), http.StatusConflict)
		return
	}
	
	// Execute load test in background
	go func() {
		ctx := context.Background()
		if err := s.executor.Execute(ctx, req); err != nil {
			log.Printf("Error executing load test '%s': %v", req.Name, err)
		}
	}()
	
	totalCalls := req.CallsPerThread * req.Threads
	
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":  "started",
		"message": fmt.Sprintf("Load test '%s' started. Results will be saved to loadtest_logs_%s and loadtest_metrics", req.Name, req.Name),
		"name":    req.Name,
		"config": map[string]interface{}{
			"threads":        req.Threads,
			"callsPerThread": req.CallsPerThread,
			"totalCalls":     totalCalls,
		},
	}
	json.NewEncoder(w).Encode(response)
	
	log.Printf("Load test '%s' started: %d threads x %d calls = %d total requests", 
		req.Name, req.Threads, req.CallsPerThread, totalCalls)
}

func (s *LoadTestServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *LoadTestServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("→ %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("← %s %s [%v]", r.Method, r.URL.Path, time.Since(start))
	})
}