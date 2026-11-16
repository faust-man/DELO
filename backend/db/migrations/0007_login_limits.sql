CREATE TABLE IF NOT EXISTS login_limits(key text PRIMARY KEY,attempts integer NOT NULL,until_at timestamptz NOT NULL);
