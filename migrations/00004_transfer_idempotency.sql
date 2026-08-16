-- +goose Up
CREATE TABLE transfer_idempotency (
	sender_id UUID NOT NULL REFERENCES accounts(id),
	idempotency_key VARCHAR(128) NOT NULL,
	request_fingerprint CHAR(64) NOT NULL,
	status VARCHAR(20) NOT NULL DEFAULT 'processing',
	transaction_id UUID UNIQUE REFERENCES transactions(id),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (sender_id, idempotency_key),
	CONSTRAINT transfer_idempotency_status_check
		CHECK (status IN ('processing', 'completed')),
	CONSTRAINT transfer_idempotency_result_check
		CHECK (
			(status = 'processing' AND transaction_id IS NULL)
			OR (status = 'completed' AND transaction_id IS NOT NULL)
		)
);

-- +goose Down
DROP TABLE IF EXISTS transfer_idempotency;
