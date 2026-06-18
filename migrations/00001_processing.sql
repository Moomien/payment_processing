-- +goose Up
CREATE TABLE IF NOT EXISTS accounts (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAUL 'user',
	balance NUMERIC(36, 18) NOT NULL DEFAULT 0, 
	CONSTRAINT balance_is_positive CHECK (balance >= 0) 
);

CREATE TABLE IF NOT EXISTS transactions (
	id UUID PRIMARY KEY, 
  	amount NUMERIC(36, 18) NOT NULL,
  	sender_id UUID NOT NULL REFERENCES accounts(id),
  	receiver_id UUID NOT NULL REFERENCES accounts(id),
  	status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'completed', 'failed')),
  	created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS refresh_token (
	jti TEXT PRIMARY KEY NOT NULL,
	user_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	revoked BOOLEAN NOT NULL DEFAULT FALSE,
	expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now() 
);

-- +goose Down
DROP TABLE IF EXISTS refresh_token;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS accounts;
