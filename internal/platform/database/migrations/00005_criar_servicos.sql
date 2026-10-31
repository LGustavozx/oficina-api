-- +goose Up
CREATE TABLE servicos (
    id                   UUID        PRIMARY KEY,
    nome                 CITEXT      NOT NULL UNIQUE,
    descricao            TEXT        NOT NULL DEFAULT '',
    preco_centavos       BIGINT      NOT NULL CHECK (preco_centavos > 0),
    duracao_estimada_min INTEGER     NOT NULL CHECK (duracao_estimada_min > 0),
    criado_em            TIMESTAMPTZ NOT NULL,
    atualizado_em        TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE servicos;
