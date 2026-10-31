-- +goose Up
CREATE TABLE ordens_servico (
    id            UUID        PRIMARY KEY,
    numero        BIGINT      GENERATED ALWAYS AS IDENTITY UNIQUE,
    cliente_id    UUID        NOT NULL REFERENCES clientes (id),
    veiculo_id    UUID        NOT NULL REFERENCES veiculos (id),
    descricao     TEXT        NOT NULL,
    status        TEXT        NOT NULL CHECK (status IN (
                      'RECEBIDA', 'EM_DIAGNOSTICO', 'AGUARDANDO_APROVACAO',
                      'EM_EXECUCAO', 'FINALIZADA', 'ENTREGUE', 'CANCELADA')),
    criado_em     TIMESTAMPTZ NOT NULL,
    atualizado_em TIMESTAMPTZ NOT NULL,
    iniciada_em   TIMESTAMPTZ,
    finalizada_em TIMESTAMPTZ,
    entregue_em   TIMESTAMPTZ
);

CREATE INDEX ordens_servico_status_idx ON ordens_servico (status, numero DESC);

CREATE TABLE itens_ordem_servico (
    ordem_id                UUID    NOT NULL REFERENCES ordens_servico (id) ON DELETE CASCADE,
    posicao                 INTEGER NOT NULL,
    tipo                    TEXT    NOT NULL CHECK (tipo IN ('SERVICO', 'PECA')),
    servico_id              UUID    REFERENCES servicos (id),
    peca_id                 UUID    REFERENCES pecas (id),
    nome                    TEXT    NOT NULL,
    preco_unitario_centavos BIGINT  NOT NULL CHECK (preco_unitario_centavos > 0),
    quantidade              INTEGER NOT NULL CHECK (quantidade > 0),
    PRIMARY KEY (ordem_id, posicao),
    CHECK ((tipo = 'SERVICO' AND servico_id IS NOT NULL AND peca_id IS NULL)
        OR (tipo = 'PECA' AND peca_id IS NOT NULL AND servico_id IS NULL))
);

-- +goose Down
DROP TABLE itens_ordem_servico;
DROP TABLE ordens_servico;
