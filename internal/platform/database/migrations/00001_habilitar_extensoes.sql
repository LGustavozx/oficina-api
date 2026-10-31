-- +goose Up
-- citext: texto sem diferença de maiúsculas/minúsculas, para o e-mail de login.
CREATE EXTENSION IF NOT EXISTS citext;

-- +goose Down
DROP EXTENSION IF EXISTS citext;
