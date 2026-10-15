package database

import (
	"dwc-server-advanced/common"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

const legacyDisabledGamesFile = "disabled_games.json"

var ErrDuplicateDisabledGame = errors.New("disabled game already exists")

type PanelDisabledGame struct {
	common.DisabledGame
	Moderator string    `json:"moderator,omitempty"`
	CreatedAt time.Time `json:"created"`
	UpdatedAt time.Time `json:"updated"`
}

func (c *Connection) UpdateDisabledGamesTable() error {
	_, err := c.pool.Exec(c.ctx, `
		CREATE TABLE IF NOT EXISTS public.disabled_games (
			id serial PRIMARY KEY,
			game_code_prefix character varying NOT NULL DEFAULT ''::character varying,
			game_name character varying NOT NULL DEFAULT ''::character varying,
			nas_return_code character varying NOT NULL DEFAULT '110'::character varying,
			error_code integer NOT NULL DEFAULT 0,
			message text NOT NULL DEFAULT ''::text,
			moderator character varying NOT NULL DEFAULT ''::character varying,
			created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
			updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
		);

		CREATE UNIQUE INDEX IF NOT EXISTS disabled_games_code_prefix_unique_idx
			ON public.disabled_games (game_code_prefix)
			WHERE game_code_prefix <> '';

		CREATE UNIQUE INDEX IF NOT EXISTS disabled_games_game_name_unique_idx
			ON public.disabled_games (game_name)
			WHERE game_name <> '';
	`)
	return err
}

// ImportLegacyDisabledGames moves entries from the old disabled_games.json
// into the database, then renames the file so it is not imported again.
func (c *Connection) ImportLegacyDisabledGames() (int, error) {
	data, err := os.ReadFile(legacyDisabledGamesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var file struct {
		Games []common.DisabledGame `json:"games"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return 0, err
	}

	imported := 0
	for _, game := range file.Games {
		tag, err := c.pool.Exec(c.ctx, `
			INSERT INTO public.disabled_games (game_code_prefix, game_name, nas_return_code, error_code, message, moderator)
			VALUES ($1, $2, $3, $4, $5, 'disabled_games.json')
			ON CONFLICT DO NOTHING
		`, game.GameCodePrefix, game.GameName, game.NASReturnCode, game.ErrorCode, game.Message)
		if err != nil {
			return imported, err
		}
		imported += int(tag.RowsAffected())
	}

	if err := os.Rename(legacyDisabledGamesFile, legacyDisabledGamesFile+".imported"); err != nil {
		return imported, err
	}
	return imported, nil
}

func (c *Connection) ListDisabledGames() ([]PanelDisabledGame, error) {
	rows, err := c.pool.Query(c.ctx, `
		SELECT id, game_code_prefix, game_name, nas_return_code, error_code, message, moderator, created_at, updated_at
		FROM public.disabled_games
		ORDER BY game_name, game_code_prefix, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	games := []PanelDisabledGame{}
	for rows.Next() {
		game, err := scanDisabledGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, game)
	}
	return games, rows.Err()
}

func (c *Connection) CreateDisabledGame(game common.DisabledGame, moderator string) (PanelDisabledGame, error) {
	row := c.pool.QueryRow(c.ctx, `
		INSERT INTO public.disabled_games (game_code_prefix, game_name, nas_return_code, error_code, message, moderator)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, game_code_prefix, game_name, nas_return_code, error_code, message, moderator, created_at, updated_at
	`, game.GameCodePrefix, game.GameName, game.NASReturnCode, game.ErrorCode, game.Message, moderator)

	created, err := scanDisabledGame(row)
	return created, mapDisabledGameError(err)
}

func (c *Connection) UpdateDisabledGame(game common.DisabledGame, moderator string) (PanelDisabledGame, bool, error) {
	row := c.pool.QueryRow(c.ctx, `
		UPDATE public.disabled_games
		SET game_code_prefix = $2,
		    game_name = $3,
		    nas_return_code = $4,
		    error_code = $5,
		    message = $6,
		    moderator = $7,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
		RETURNING id, game_code_prefix, game_name, nas_return_code, error_code, message, moderator, created_at, updated_at
	`, game.ID, game.GameCodePrefix, game.GameName, game.NASReturnCode, game.ErrorCode, game.Message, moderator)

	updated, err := scanDisabledGame(row)
	if err == pgx.ErrNoRows {
		return PanelDisabledGame{}, false, nil
	}
	if err != nil {
		return PanelDisabledGame{}, false, mapDisabledGameError(err)
	}
	return updated, true, nil
}

func (c *Connection) DeleteDisabledGame(id int) (PanelDisabledGame, bool, error) {
	row := c.pool.QueryRow(c.ctx, `
		DELETE FROM public.disabled_games
		WHERE id = $1
		RETURNING id, game_code_prefix, game_name, nas_return_code, error_code, message, moderator, created_at, updated_at
	`, id)

	deleted, err := scanDisabledGame(row)
	if err == pgx.ErrNoRows {
		return PanelDisabledGame{}, false, nil
	}
	if err != nil {
		return PanelDisabledGame{}, false, err
	}
	return deleted, true, nil
}

// ReloadDisabledGamesCache pushes the current table contents into the
// in-memory list used by NAS and GPCM.
func (c *Connection) ReloadDisabledGamesCache() error {
	games, err := c.ListDisabledGames()
	if err != nil {
		return err
	}

	cached := make([]common.DisabledGame, 0, len(games))
	for _, game := range games {
		cached = append(cached, game.DisabledGame)
	}
	common.SetDisabledGames(cached)
	return nil
}

func scanDisabledGame(row interface {
	Scan(dest ...interface{}) error
}) (PanelDisabledGame, error) {
	game := PanelDisabledGame{}
	err := row.Scan(
		&game.ID,
		&game.GameCodePrefix,
		&game.GameName,
		&game.NASReturnCode,
		&game.ErrorCode,
		&game.Message,
		&game.Moderator,
		&game.CreatedAt,
		&game.UpdatedAt,
	)
	return game, err
}

func mapDisabledGameError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicateDisabledGame
	}
	return err
}
