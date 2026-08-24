package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-commerce/config"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// A *sqlx.DB is already a pool, and opening a second one against the same
// database doubles the connection budget for no gain. GetConnection resolves
// exactly one, however many callers ask; NewConnection stays exported for tests
// that want a throwaway pool of their own.
var (
	once     sync.Once
	instance *sqlx.DB
	openErr  error
)

// Pool limits. They are deliberately conservative: a hosted Postgres usually
// caps connections well below what an unbounded pool will happily open.
const (
	maxOpenConns    = 25
	maxIdleConns    = 5
	connMaxLifetime = 30 * time.Minute
	connMaxIdleTime = 5 * time.Minute
	connectTimeout  = 10 * time.Second
)

// getConnectionString builds a libpq connection string from Postgres config.
func getConnectionString(pg *config.PGConfig) string {
	return fmt.Sprintf(
		"host=%s dbname=%s user=%s password=%s sslmode=%s",
		pg.Host, pg.Database, pg.User, pg.Password, pg.SslMode,
	)
}

// GetConnection returns the process-wide Postgres pool, opening it on the first
// call. Like config.LoadConfig it caches the failure too, so a later caller
// cannot silently get a different answer than the first one did.
func GetConnection(pg *config.PGConfig) (*sqlx.DB, error) {
	once.Do(func() {
		instance, openErr = NewConnection(pg)
	})
	return instance, openErr
}

// NewConnection opens a Postgres connection pool using the given config and
// verifies it can actually be reached before handing it back.
func NewConnection(pg *config.PGConfig) (*sqlx.DB, error) {
	if pg == nil {
		return nil, fmt.Errorf("db: no postgres configuration provided")
	}

	dbCon, err := sqlx.Open("postgres", getConnectionString(pg))
	if err != nil {
		return nil, fmt.Errorf("opening postgres: %w", err)
	}

	dbCon.SetMaxOpenConns(maxOpenConns)
	dbCon.SetMaxIdleConns(maxIdleConns)
	dbCon.SetConnMaxLifetime(connMaxLifetime)
	dbCon.SetConnMaxIdleTime(connMaxIdleTime)

	// sqlx.Open is lazy. Ping so a bad host or password fails at startup, where
	// it is one clear log line, instead of on the first request.
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	if err := dbCon.PingContext(ctx); err != nil {
		dbCon.Close()
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}

	return dbCon, nil
}
