-- +goose Up
CREATE TABLE veiculos (
    id            UUID        PRIMARY KEY,
    cliente_id    UUID        NOT NULL REFERENCES clientes (id),
    placa         TEXT        NOT NULL UNIQUE,
    marca         TEXT        NOT NULL,
    modelo        TEXT        NOT NULL,
    ano           INTEGER     NOT NULL CHECK (ano >= 1900),
    criado_em     TIMESTAMPTZ NOT NULL,
    atualizado_em TIMESTAMPTZ NOT NULL
);

CREATE INDEX veiculos_cliente_id_idx ON veiculos (cliente_id);

-- +goose Down
DROP TABLE veiculos;
