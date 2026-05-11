package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sirupsen/logrus"

	_ "github.com/lib/pq"
)

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type DB struct {
	*sql.DB
	DBTX Querier
}

func NewDB(connStr string) (*DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to the database: %w", err)
	}

	logrus.Info("Successfully connected to the database.")
	return &DB{DB: db, DBTX: db}, nil
}

func (db *DB) Close() {
	if db.DB != nil {
		db.DB.Close()
		logrus.Info("Database connection closed.")
	}
}
