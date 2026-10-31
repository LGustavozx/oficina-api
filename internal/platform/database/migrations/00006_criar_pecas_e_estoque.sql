-- +goose Up
CREATE TABLE pecas (
    id                      UUID        PRIMARY KEY,
    sku                     TEXT        NOT NULL UNIQUE,
    nome                    TEXT        NOT NULL,
    descricao               TEXT        NOT NULL DEFAULT '',
    preco_unitario_centavos BIGINT      NOT NULL CHECK (preco_unitario_centavos > 0),
    quantidade              INTEGER     NOT NULL DEFAULT 0 CHECK (quantidade >= 0 AND quantidade <= 1000000),
    criado_em               TIMESTAMPTZ NOT NULL,
    atualizado_em           TIMESTAMPTZ NOT NULL
);

CREATE TABLE movimentacoes_estoque (
    id         UUID        PRIMARY KEY,
    peca_id    UUID        NOT NULL REFERENCES pecas (id),
    tipo       TEXT        NOT NULL CHECK (tipo IN ('ENTRADA', 'SAIDA')),
    quantidade INTEGER     NOT NULL CHECK (quantidade > 0),
    motivo     TEXT        NOT NULL,
    criado_em  TIMESTAMPTZ NOT NULL
);

CREATE INDEX movimentacoes_estoque_peca_idx ON movimentacoes_estoque (peca_id, criado_em DESC);

-- +goose Down
DROP TABLE movimentacoes_estoque;
DROP TABLE pecas;
