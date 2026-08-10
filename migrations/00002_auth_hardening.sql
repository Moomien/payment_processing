-- +goose Up

CREATE UNIQUE INDEX IF NOT EXISTS accounts_email_lower_uidx
	ON accounts (LOWER(email));

CREATE INDEX IF NOT EXISTS refresh_token_user_id_idx
	ON refresh_token (user_id);

CREATE INDEX IF NOT EXISTS refresh_token_expires_at_idx
	ON refresh_token (expires_at);

-- +goose Down
DROP INDEX IF EXISTS refresh_token_expires_at_idx;
DROP INDEX IF EXISTS refresh_token_user_id_idx;
DROP INDEX IF EXISTS accounts_email_lower_uidx;
