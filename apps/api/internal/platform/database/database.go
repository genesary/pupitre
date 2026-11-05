// Package database opens the connection pool the domains share.
package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Connect opens a GORM connection to connURL and verifies connectivity with
// a ping before returning it. maxOpenConns and maxIdleConns cap the
// connection pool size.
//
// Three settings past the pool size matter under sustained load:
//
//   - PrepareStmt caches a prepared statement per distinct SQL string, so
//     the repeated queries that make up a request are parsed and planned
//     once per connection rather than on every execution.
//   - ConnMaxLifetime recycles connections, which keeps a long-lived pool
//     from pinning itself to backends that a failover or a rolling restart
//     has moved on from.
//   - ConnMaxIdleTime returns connections the service is no longer using,
//     so an idle replica does not hold a share of the database's
//     connection limit away from a busy one.
func Connect(ctx context.Context, connURL string, maxOpenConns, maxIdleConns int) (*gorm.DB, error) {
	gdb, err := gorm.Open(postgres.Open(connURL), &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(gormlogger.Warn),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("unwrap sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	err = sqlDB.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return gdb, nil
}

// connMaxLifetime bounds how long a pooled connection is reused before
// being closed and reopened.
const connMaxLifetime = 30 * time.Minute

// connMaxIdleTime bounds how long an unused pooled connection is held.
const connMaxIdleTime = 5 * time.Minute
