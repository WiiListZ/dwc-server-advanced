package database

import (
	"encoding/json"

	"github.com/jackc/pgx/v4"
)

// UpdateSettingsTable creates the key/value table for server settings that
// live in the database rather than in config.yml.
func (c *Connection) UpdateSettingsTable() error {
	_, err := c.pool.Exec(c.ctx, `
		CREATE TABLE IF NOT EXISTS public.panel_settings (
			key character varying PRIMARY KEY,
			value jsonb NOT NULL,
			updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// GetSetting decodes the stored value into dst. It reports false when unset.
func (c *Connection) GetSetting(key string, dst any) (bool, error) {
	var raw []byte
	err := c.pool.QueryRow(c.ctx, `SELECT value FROM public.panel_settings WHERE key = $1`, key).Scan(&raw)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(raw, dst)
}

// SetSetting stores value as JSON; a nil value deletes the key.
func (c *Connection) SetSetting(key string, value any) error {
	if value == nil {
		_, err := c.pool.Exec(c.ctx, `DELETE FROM public.panel_settings WHERE key = $1`, key)
		return err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = c.pool.Exec(c.ctx, `
		INSERT INTO public.panel_settings (key, value, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP
	`, key, raw)
	return err
}
