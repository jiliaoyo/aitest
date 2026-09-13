-- +goose Up

ALTER TABLE users
    ADD COLUMN furigana_size smallint NOT NULL DEFAULT 70,
    ADD CONSTRAINT users_furigana_size_check CHECK (furigana_size BETWEEN 50 AND 100);

-- +goose Down

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_furigana_size_check;
ALTER TABLE users DROP COLUMN IF EXISTS furigana_size;
