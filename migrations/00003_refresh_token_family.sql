-- +goose Up

ALTER TABLE refresh_token
	ADD COLUMN IF NOT EXISTS family_id UUID;

UPDATE refresh_token
SET family_id = gen_random_uuid()
WHERE family_id IS NULL;

ALTER TABLE refresh_token
	ALTER COLUMN family_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS refresh_token_family_id_idx
	ON refresh_token (family_id);

-- +goose Down

DROP INDEX IF EXISTS refresh_token_family_id_idx;

ALTER TABLE refresh_token
	DROP COLUMN IF EXISTS family_id;
