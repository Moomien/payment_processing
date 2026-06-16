-- +goose Up
CREATE TABLE IF NOT EXISTS accounts ( 
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	balance NUMERIC(36, 18) NOT NULL DEFAULT 0, 
	CONSTRAINT balance_is_positive CHECK (balance >= 0) 
);

CREATE TABLE IF NOT EXISTS transactions (
	id UUID PRIMARY KEY, 
  	amount NUMERIC(36, 18) NOT NULL,
  	sender_id UUID NOT NULL REFERENCES accounts(id),
  	receiver_id UUID NOT NULL REFERENCES accounts(id),
  	status VARCHAR(20) NOT NULL DEFAULT 'pending',
  	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);


-- +goose Down
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS accounts;
