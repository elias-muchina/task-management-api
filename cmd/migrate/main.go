package main

import (
	"context"
	"fmt"
	"log"

	"task-manager-api/internal/config"

	"github.com/jackc/pgx/v5"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Connect to PostgreSQL
	ctx := context.Background()

	// Connect without using the pool for migrations
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port,
		cfg.Database.DBName, cfg.Database.SSLMode,
	)

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	// Run migrations
	if err := runMigrations(ctx, conn); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("✅ Migrations completed successfully")
}

func runMigrations(ctx context.Context, conn *pgx.Conn) error {
	// Create users table
	usersTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`

	// Create tasks table
	tasksTableSQL := `
		CREATE TABLE IF NOT EXISTS tasks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			description TEXT,
			status VARCHAR(50) DEFAULT 'pending',
			priority INTEGER DEFAULT 1,
			due_date TIMESTAMP,
			completed_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`

	// Create indexes
	indexesSQL := []string{
		"CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)",
		"CREATE INDEX IF NOT EXISTS idx_tasks_due_date ON tasks(due_date)",
	}

	// Execute migrations
	log.Println("Running migrations...")

	// Create users table
	if _, err := conn.Exec(ctx, usersTableSQL); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}
	log.Println("✅ Created users table")

	// Create tasks table
	if _, err := conn.Exec(ctx, tasksTableSQL); err != nil {
		return fmt.Errorf("failed to create tasks table: %w", err)
	}
	log.Println("✅ Created tasks table")

	// Create indexes
	for i, indexSQL := range indexesSQL {
		if _, err := conn.Exec(ctx, indexSQL); err != nil {
			return fmt.Errorf("failed to create index %d: %w", i+1, err)
		}
	}
	log.Println("✅ Created indexes")

	return nil
}
