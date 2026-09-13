-- +goose Up

ALTER TABLE users ADD COLUMN show_furigana boolean NOT NULL DEFAULT true;

-- +goose Down

ALTER TABLE users DROP COLUMN IF EXISTS show_furigana;
