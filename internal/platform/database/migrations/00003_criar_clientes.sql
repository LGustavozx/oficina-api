-- +goose Up
CREATE TABLE clientes (
    id             UUID        PRIMARY KEY,
    nome           TEXT        NOT NULL,
    documento      TEXT        NOT NULL UNIQUE,
    tipo_documento TEXT        NOT NULL CHECK (tipo_documento IN ('CPF', 'CNPJ')),
    email          CITEXT      NOT NULL DEFAULT '',
    telefone       TEXT        NOT NULL DEFAULT '',
    criado_em      TIMESTAMPTZ NOT NULL,
    atualizado_em  TIMESTAMPTZ NOT NULL
);

CREATE INDEX clientes_nome_idx ON clientes (nome);

-- +goose Down
DROP TABLE clientes;
