-- +goose Up
-- citext: tipo de texto que ignora maiúsculas/minúsculas. Será usado no e-mail
-- de login (fase 2), para "Admin@x.com" e "admin@x.com" serem a mesma conta.
CREATE EXTENSION IF NOT EXISTS citext;

-- +goose Down
DROP EXTENSION IF EXISTS citext;
