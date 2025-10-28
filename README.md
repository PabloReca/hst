# HTS - HTTP SRE Toolkit

Site Reliability Engineering toolkit for monitoring and load testing HTTP endpoints.

## Features

### Health Checks
- Periodic endpoint monitoring with configurable intervals
- HTTP method and header support
- Expected status code validation
- Optional response body validation
- Real-time log streaming
- Automatic metric collection

### Load Testing
- Concurrent multi-threaded load testing
- Configurable request patterns (threads × calls per thread)
- Comprehensive performance metrics:
  - Requests per second (RPS)
  - Response time distribution (min, median, p95, p99, max)
  - Success rate tracking
  - Status code distribution
  - Throughput measurement (MB/s)
- Individual request logging
- Custom headers and body support

## Setup

### Prerequisites
- Docker & Docker Compose
- Go 1.21+ (for local development)
- Node.js 24+ & pnpm (for local development)

### Local Development

**Backend**
```bash
cd backend
go mod download
go run .
```

**Portal**
```bash
cd portal
pnpm install
pnpm dev