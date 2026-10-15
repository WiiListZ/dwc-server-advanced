package database

import (
	"context"
	"dwc-server-advanced/common"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Connection struct {
	pool *pgxpool.Pool
	ctx  context.Context
}

func Start(config common.Config) Connection {
	conn := Connection{
		ctx: context.Background(),
	}

	dbString := fmt.Sprintf("postgres://%s:%s@%s/%s", config.Username, config.Password, config.DatabaseAddress, config.DatabaseName)
	dbConf, err := pgxpool.ParseConfig(dbString)
	if err != nil {
		panic(err)
	}

	conn.pool, err = pgxpool.ConnectConfig(conn.ctx, dbConf)
	if err != nil {
		panic(err)
	}

	return conn
}

func (c *Connection) Close() {
	if c != nil && c.pool != nil {
		c.pool.Close()
	}
}

// TryConnect opens a connection and pings the database, returning an error
// instead of panicking. The setup wizard uses it to test credentials.
func TryConnect(config common.Config, timeout time.Duration) (Connection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	dbString := fmt.Sprintf("postgres://%s:%s@%s/%s",
		url.PathEscape(config.Username), url.PathEscape(config.Password), config.DatabaseAddress, url.PathEscape(config.DatabaseName))
	dbConf, err := pgxpool.ParseConfig(dbString)
	if err != nil {
		return Connection{}, err
	}
	pool, err := pgxpool.ConnectConfig(ctx, dbConf)
	if err != nil {
		return Connection{}, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return Connection{}, err
	}
	return Connection{pool: pool, ctx: context.Background()}, nil
}

// HasSchema reports whether schema.sql has been applied (the users table exists).
func (c *Connection) HasSchema() (bool, error) {
	var exists bool
	err := c.pool.QueryRow(c.ctx, `SELECT to_regclass('public.users') IS NOT NULL`).Scan(&exists)
	return exists, err
}

// ApplySchema runs schema.sql on an empty database. schema.sql gives the
// tables to CURRENT_USER, so they are owned by the user the server connects
// with, whatever its name.
func (c *Connection) ApplySchema(schema string) error {
	conn, err := c.pool.Acquire(c.ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	// Simple protocol so the whole multi-statement file runs in one call.
	_, err = conn.Conn().PgConn().Exec(c.ctx, schema).ReadAll()
	return err
}
