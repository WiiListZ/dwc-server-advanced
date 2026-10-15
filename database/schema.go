package database

func (c *Connection) UpdateTables() {
	_, _ = c.pool.Exec(c.ctx, `

	ALTER TABLE ONLY public.users
		ADD IF NOT EXISTS last_ip_address character varying DEFAULT ''::character varying,
		ADD IF NOT EXISTS last_ingamesn character varying DEFAULT ''::character varying,
		ADD IF NOT EXISTS has_ban boolean DEFAULT false,
		ADD IF NOT EXISTS ban_issued timestamp without time zone,
		ADD IF NOT EXISTS ban_expires timestamp without time zone,
		ADD IF NOT EXISTS ban_revoked timestamp without time zone,
		ADD IF NOT EXISTS ban_reason character varying,
		ADD IF NOT EXISTS ban_reason_hidden character varying,
		ADD IF NOT EXISTS ban_moderator character varying,
		ADD IF NOT EXISTS ban_tos boolean,
		ADD IF NOT EXISTS open_host boolean DEFAULT true,
		ADD IF NOT EXISTS open_host_mode character varying DEFAULT 'disabled',
		ADD IF NOT EXISTS open_host_list bigint[] DEFAULT ARRAY[]::bigint[],
		ADD IF NOT EXISTS open_host_list_display text[] DEFAULT ARRAY[]::text[],
		ADD IF NOT EXISTS right_open_host boolean DEFAULT true,
		ADD IF NOT EXISTS allow_default_keys boolean DEFAULT false;
	`)

	// Backfill open_host_mode from legacy open_host bool. Guarded by NULL so
	// the rewrite runs once; subsequent edits to open_host_mode persist.
	_, _ = c.pool.Exec(c.ctx, `
	UPDATE public.users
	SET open_host_mode = CASE WHEN open_host THEN 'blacklist' ELSE 'disabled' END
	WHERE open_host_mode IS NULL OR open_host_mode = '';
	`)

	// Default mode flipped from blacklist to disabled. Idempotent: only
	// changes the column default when it still says 'blacklist'.
	_, _ = c.pool.Exec(c.ctx, `
	DO $openhostmode$
	BEGIN
		IF (SELECT column_default FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'open_host_mode') = '''blacklist''::character varying' THEN
			ALTER TABLE public.users ALTER COLUMN open_host_mode SET DEFAULT 'disabled';
		END IF;
	END
	$openhostmode$;
	`)

	// open_host became a default-on right; flip the column default and
	// backfill rows that still carry the legacy default-false. Guarded by
	// the current column_default so this only runs once.
	_, _ = c.pool.Exec(c.ctx, `
	DO $openhost$
	BEGIN
		IF (SELECT column_default FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'open_host') = 'false' THEN
			ALTER TABLE public.users ALTER COLUMN open_host SET DEFAULT true;
			UPDATE public.users SET open_host = true WHERE open_host = false;
		END IF;
	END
	$openhost$;
	`)

	_, _ = c.pool.Exec(c.ctx, `

	DO $$ 
	BEGIN
    	IF (SELECT data_type FROM information_schema.columns WHERE table_name='users' AND column_name='ng_device_id') != 'ARRAY' THEN
        	ALTER TABLE public.users
            	ALTER COLUMN ng_device_id TYPE bigint[] using array[ng_device_id];
    	END IF;
	END $$;

	`)

	_, _ = c.pool.Exec(c.ctx, `

	ALTER TABLE ONLY public.mario_kart_wii_sake
        ADD IF NOT EXISTS id serial PRIMARY KEY,
		ADD IF NOT EXISTS upload_time timestamp without time zone;
	
	`)

	_ = c.UpdatePanelTables()
}
