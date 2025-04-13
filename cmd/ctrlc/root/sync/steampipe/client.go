package steampipe

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type Client struct {
	Connection string `default:"connection"`
	db         *sql.DB
}

func NewSteampipeClient(spConnection string) (*Client, error) {
	client := &Client{
		Connection: spConnection,
	}

	// Initialize database connection
	db, err := sql.Open("postgres", spConnection)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	client.db = db
	return client, nil
}

// Close closes the database connection
func (c *Client) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// GetDB returns the underlying database connection
func (c *Client) GetDB() *sql.DB {
	return c.db
}
