--
-- PostgreSQL database dump
--

-- Dumped from database version 14.9 (Ubuntu 14.9-0ubuntu0.22.04.1)
-- Dumped by pg_dump version 14.9 (Homebrew)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: users; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.users (
    profile_id bigint NOT NULL,
    user_id bigint NOT NULL,
    gsbrcd character varying NOT NULL,
    password character varying NOT NULL,
    ng_device_id bigint,
    email character varying NOT NULL,
    unique_nick character varying NOT NULL,
    firstname character varying,
    lastname character varying DEFAULT ''::character varying
);


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
	ADD IF NOT EXISTS right_open_host boolean DEFAULT true;

--
-- Change ng_device_id from bigint to bigint[]
--
DO $$ 
BEGIN
    IF (SELECT data_type FROM information_schema.columns WHERE table_name='users' AND column_name='ng_device_id') != 'ARRAY' THEN
        ALTER TABLE public.users
            ALTER COLUMN ng_device_id TYPE bigint[] using array[ng_device_id];
    END IF;
END $$;

ALTER TABLE public.users OWNER TO CURRENT_USER;

--
-- Name: sake_records; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.sake_records (
    game_id integer NOT NULL,
    table_id character varying NOT NULL,
    record_id integer NOT NULL DEFAULT (random() * 2147483647)::integer,
    owner_id integer NOT NULL,
    fields jsonb NOT NULL CHECK (jsonb_typeof(fields) = 'object' AND jsonb_array_length(jsonb_path_query_array(fields, '$.keyvalue().key')) <= 64),
    create_time timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    update_time timestamp without time zone DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT one_sake_record_constraint UNIQUE (game_id, table_id, record_id)
);

--
-- Name: mario_kart_wii_sake; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.mario_kart_wii_sake (
    regionid smallint NOT NULL CHECK (regionid >= 1 AND regionid <= 7),
    courseid smallint NOT NULL CHECK (courseid >= 0 AND courseid <= 32767),
    score integer NOT NULL CHECK (score > 0 AND score < 360000),
    pid integer NOT NULL CHECK (pid > 0),
    playerinfo varchar(108) NOT NULL CHECK (LENGTH(playerinfo) = 108),
    ghost bytea CHECK (ghost IS NULL OR (OCTET_LENGTH(ghost) BETWEEN 148 AND 10240)),

    CONSTRAINT one_time_per_course_constraint UNIQUE (courseid, pid)
);


ALTER TABLE ONLY public.mario_kart_wii_sake
    ADD IF NOT EXISTS id serial PRIMARY KEY,
    ADD IF NOT EXISTS upload_time timestamp without time zone;


ALTER TABLE public.mario_kart_wii_sake OWNER TO CURRENT_USER;

--
-- Name: gamestats_public_data; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.gamestats_public_data (
    profile_id bigint NOT NULL,
    dindex character varying NOT NULL,
    ptype character varying NOT NULL,
    pdata character varying NOT NULL,
    modified_time timestamp without time zone NOT NULL,

    CONSTRAINT one_pdata_constraint UNIQUE (profile_id, dindex, ptype)
);

--
-- Name: users_profile_id_seq; Type: SEQUENCE; Schema: public; Owner: dwc-server-advanced
--

CREATE SEQUENCE IF NOT EXISTS public.users_profile_id_seq
    AS integer
    START WITH 1000000000
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.users_profile_id_seq OWNER TO CURRENT_USER;

--
-- Name: users_profile_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: dwc-server-advanced
--

ALTER SEQUENCE public.users_profile_id_seq OWNED BY public.users.profile_id;

--
-- Name: users profile_id; Type: DEFAULT; Schema: public; Owner: dwc-server-advanced
--

ALTER TABLE ONLY public.users ALTER COLUMN profile_id SET DEFAULT nextval('public.users_profile_id_seq'::regclass);

--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: dwc-server-advanced
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (profile_id);

--
-- Name: events; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.events (
    id serial PRIMARY KEY,
    event_type character varying NOT NULL,
    event_data jsonb NOT NULL,
    event_time timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

--
-- Name: panel_accounts; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.panel_accounts (
    id serial PRIMARY KEY,
    username character varying NOT NULL UNIQUE,
    password_hash bytea NOT NULL,
    role character varying NOT NULL CHECK (role IN ('owner', 'moderator', 'player')),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    profile_id bigint,
    linked_at timestamp without time zone
);

CREATE UNIQUE INDEX IF NOT EXISTS panel_accounts_profile_id_unique_idx
    ON public.panel_accounts (profile_id)
    WHERE profile_id IS NOT NULL;

ALTER TABLE public.panel_accounts
    ADD IF NOT EXISTS can_manage_rooms_end_game boolean NOT NULL DEFAULT false;

ALTER TABLE public.panel_accounts OWNER TO CURRENT_USER;

--
-- Name: consoles; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.consoles (
    id serial PRIMARY KEY,
    ng_device_id bigint NOT NULL UNIQUE,
    account_id integer REFERENCES public.panel_accounts(id) ON DELETE SET NULL,
    primary_profile_id bigint,
    console_type text NOT NULL DEFAULT 'Wii',
    console_friend_code text NOT NULL DEFAULT ''::text,
    display_name text NOT NULL DEFAULT ''::text,
    has_ban boolean NOT NULL DEFAULT false,
    ban_tos boolean,
    ban_issued timestamp without time zone,
    ban_expires timestamp without time zone,
    ban_revoked timestamp without time zone,
    ban_reason text,
    ban_reason_hidden text,
    ban_moderator text,
    ban_ingame_name text,
    ban_profile_id bigint,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    linked_at timestamp without time zone,
    last_login_at timestamp without time zone
);

CREATE INDEX IF NOT EXISTS consoles_account_idx
    ON public.consoles (account_id, linked_at DESC, id)
    WHERE account_id IS NOT NULL;

ALTER TABLE public.consoles OWNER TO CURRENT_USER;

ALTER TABLE ONLY public.users
    ADD IF NOT EXISTS console_id integer REFERENCES public.consoles(id);

CREATE INDEX IF NOT EXISTS users_console_id_idx
    ON public.users (console_id);

ALTER TABLE ONLY public.panel_accounts
    ADD IF NOT EXISTS console_id integer REFERENCES public.consoles(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS panel_accounts_console_unique_idx
    ON public.panel_accounts (console_id)
    WHERE console_id IS NOT NULL;

--
-- Name: panel_sessions; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.panel_sessions (
    token_hash character varying(64) PRIMARY KEY,
    account_id integer NOT NULL REFERENCES public.panel_accounts(id) ON DELETE CASCADE,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    expires_at timestamp without time zone NOT NULL
);

ALTER TABLE public.panel_sessions OWNER TO CURRENT_USER;

--
-- Name: panel_ban_history; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.panel_ban_history (
    id serial PRIMARY KEY,
    profile_id bigint NOT NULL,
    gsbrcd character varying NOT NULL DEFAULT ''::character varying,
    ingame_name character varying NOT NULL DEFAULT ''::character varying,
    reason character varying,
    reason_hidden character varying,
    moderator character varying,
    tos boolean,
    issued timestamp without time zone NOT NULL,
    expires timestamp without time zone,
    revoked timestamp without time zone
);

CREATE INDEX IF NOT EXISTS panel_ban_history_profile_id_idx
    ON public.panel_ban_history (profile_id);

CREATE INDEX IF NOT EXISTS panel_ban_history_visible_idx
    ON public.panel_ban_history (issued, expires, revoked);

ALTER TABLE public.panel_ban_history
    ADD IF NOT EXISTS console_id integer REFERENCES public.consoles(id) ON DELETE SET NULL,
    ADD IF NOT EXISTS ng_device_id bigint;

ALTER TABLE public.panel_ban_history OWNER TO CURRENT_USER;

--
-- Name: panel_login_messages; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.panel_login_messages (
    id serial PRIMARY KEY,
    profile_id bigint NOT NULL,
    message text NOT NULL,
    moderator character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    delivered_at timestamp without time zone
);

CREATE INDEX IF NOT EXISTS panel_login_messages_profile_pending_idx
    ON public.panel_login_messages (profile_id, delivered_at, created_at);

ALTER TABLE public.panel_login_messages OWNER TO CURRENT_USER;

--
-- Name: panel_profile_link_codes; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.panel_profile_link_codes (
    id serial PRIMARY KEY,
    account_id integer NOT NULL REFERENCES public.panel_accounts(id) ON DELETE CASCADE,
    profile_id bigint NOT NULL,
    code_hash character varying(64) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    expires_at timestamp without time zone NOT NULL,
    consumed_at timestamp without time zone
);

CREATE INDEX IF NOT EXISTS panel_profile_link_codes_pending_idx
    ON public.panel_profile_link_codes (account_id, profile_id, expires_at, consumed_at);

ALTER TABLE public.panel_profile_link_codes OWNER TO CURRENT_USER;

--
-- Name: panel_room_kick_history; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.panel_room_kick_history (
    id serial PRIMARY KEY,
    room_id character varying NOT NULL,
    game_name character varying NOT NULL DEFAULT ''::character varying,
    owner_profile_id bigint NOT NULL,
    target_profile_id bigint NOT NULL,
    target_ingame_name character varying NOT NULL DEFAULT ''::character varying,
    target_friend_code character varying NOT NULL DEFAULT ''::character varying,
    reason character varying NOT NULL,
    moderator character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS panel_room_kick_history_created_idx
    ON public.panel_room_kick_history (created_at);

ALTER TABLE public.panel_room_kick_history OWNER TO CURRENT_USER;

--
-- Name: panel_kick_history; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.panel_kick_history (
    id serial PRIMARY KEY,
    profile_id bigint NOT NULL,
    target_ingame_name character varying NOT NULL DEFAULT ''::character varying,
    target_friend_code character varying NOT NULL DEFAULT ''::character varying,
    reason character varying NOT NULL,
    moderator character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS panel_kick_history_created_idx
    ON public.panel_kick_history (created_at);

CREATE INDEX IF NOT EXISTS panel_kick_history_profile_id_idx
    ON public.panel_kick_history (profile_id, created_at DESC);

ALTER TABLE public.panel_kick_history OWNER TO CURRENT_USER;

--
-- Name: panel_room_reports; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.panel_room_reports (
    id serial PRIMARY KEY,
    room_id character varying NOT NULL,
    game_name character varying NOT NULL DEFAULT ''::character varying,
    room_type character varying NOT NULL DEFAULT ''::character varying,
    reporter_profile_id bigint NOT NULL,
    reporter_ingame_name character varying NOT NULL DEFAULT ''::character varying,
    reporter_friend_code character varying NOT NULL DEFAULT ''::character varying,
    reporter_type character varying NOT NULL DEFAULT 'user',
    target_profile_id bigint NOT NULL,
    target_ingame_name character varying NOT NULL DEFAULT ''::character varying,
    target_friend_code character varying NOT NULL DEFAULT ''::character varying,
    reason text NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS panel_room_reports_once_idx
    ON public.panel_room_reports (reporter_profile_id, target_profile_id);

CREATE INDEX IF NOT EXISTS panel_room_reports_target_idx
    ON public.panel_room_reports (target_profile_id, created_at DESC);

CREATE INDEX IF NOT EXISTS panel_room_reports_created_idx
    ON public.panel_room_reports (created_at DESC);

ALTER TABLE public.panel_room_reports OWNER TO CURRENT_USER;

--
-- Name: panel_match_logs; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

CREATE TABLE IF NOT EXISTS public.panel_match_logs (
    id serial PRIMARY KEY,
    source_key character varying NOT NULL UNIQUE,
    room_id character varying NOT NULL,
    dwc_group_id bigint NOT NULL DEFAULT 0,
    game_name character varying NOT NULL DEFAULT ''::character varying,
    match_type character varying NOT NULL DEFAULT ''::character varying,
    mkw_region character varying NOT NULL DEFAULT ''::character varying,
    race_number integer NOT NULL DEFAULT 0,
    course_id integer NOT NULL DEFAULT -1,
    engine_class_id integer NOT NULL DEFAULT -1,
    host_profile_id bigint,
    host_ingame_name character varying NOT NULL DEFAULT ''::character varying,
    player_count integer NOT NULL DEFAULT 0,
    players jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS panel_match_logs_created_idx
    ON public.panel_match_logs (created_at);

ALTER TABLE public.panel_match_logs OWNER TO CURRENT_USER;

--
-- Name: disabled_games; Type: TABLE; Schema: public; Owner: dwc-server-advanced
--

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

ALTER TABLE public.disabled_games OWNER TO CURRENT_USER;

-- 
-- PostgreSQL database dump complete
--

