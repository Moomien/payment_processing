-- +goose Up
ALTER TABLE transactions
	ADD CONSTRAINT transactions_amount_positive CHECK (amount > 0),
	ADD CONSTRAINT transactions_distinct_accounts CHECK (sender_id <> receiver_id);

-- +goose Down
ALTER TABLE transactions
	DROP CONSTRAINT IF EXISTS transactions_distinct_accounts,
	DROP CONSTRAINT IF EXISTS transactions_amount_positive;
