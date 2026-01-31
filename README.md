# Task Manager API (Go)

A production-ready REST API for task management built with Go, featuring authentication, concurrency, caching, and Docker deployment.

## 🚀 Features

- **JWT Authentication** - Secure user registration/login
- **Task CRUD Operations** - Full task management with filtering
- **Concurrent Processing** - Goroutines & channels for async operations
- **Redis Caching** - Performance optimization for frequent queries
- **Rate Limiting** - Redis-based request limiting
- **Dockerized** - Complete container setup with Docker Compose
- **Comprehensive Testing** - Unit & integration tests with 90%+ coverage
- **Graceful Shutdown** - Proper handling of interruptions
- **Structured Logging** - JSON logging for production

## 🛠️ Tech Stack

- **Go 1.24+** - Backend language
- **Gin** - HTTP web framework
- **PostgreSQL** - Primary database
- **Redis** - Caching & rate limiting
- **Docker** - Containerization
- **JWT** - Authentication
- **pgx** - PostgreSQL driver
- **Testify** - Testing framework

## 📦 Quick Start

### Prerequisites

- Go 1.24+
- Docker & Docker Compose
- Make (optional)

### Using Docker (Recommended)

```bash
# Clone the repository
git clone https://github.com/elias-muchina/task-manager-api.git
cd task-manager-api

# Copy environment variables
cp .env.example .env

# Start services
make docker-up

# API will be available at http://localhost:8080
```
