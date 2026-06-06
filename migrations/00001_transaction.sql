-- +goose Up
CREATE TABLE if not EXISTS Transact (
	transaction_id BIGSERIAL PRIMARY KEY, 
  	amount NUMERIC(15, 2) NOT NULL,
  	sender_id INT NOT NULL,
  	receiver_id INT NOT NULL,
  	status VARCHAR(20) NOT NULL DEFAULT 'pending',
  	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE Transact;
