-- +goose Up
CREATE TABLE usuarios (
    id         UUID        PRIMARY KEY,
    email      CITEXT      NOT NULL UNIQUE,
    senha_hash TEXT        NOT NULL,
    criado_em  TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE usuarios;
