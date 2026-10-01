CREATE TABLE usuarios (
    id          UUID PRIMARY KEY,
    email       TEXT        NOT NULL UNIQUE,
    senha_hash  TEXT        NOT NULL,
    perfil      TEXT        NOT NULL DEFAULT 'ADMIN',
    ativo       BOOLEAN     NOT NULL DEFAULT TRUE,
    criado_em   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE clientes (
    id          UUID PRIMARY KEY,
    nome        TEXT        NOT NULL,
    documento   TEXT        NOT NULL UNIQUE,
    tipo_pessoa TEXT        NOT NULL CHECK (tipo_pessoa IN ('PF', 'PJ')),
    email       TEXT,
    telefone    TEXT,
    ativo       BOOLEAN     NOT NULL DEFAULT TRUE,
    criado_em   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE veiculos (
    id         UUID PRIMARY KEY,
    cliente_id UUID     NOT NULL REFERENCES clientes (id),
    placa      TEXT     NOT NULL UNIQUE,
    marca      TEXT     NOT NULL,
    modelo     TEXT     NOT NULL,
    ano        SMALLINT NOT NULL CHECK (ano >= 1950),
    ativo      BOOLEAN  NOT NULL DEFAULT TRUE
);
CREATE INDEX ix_veiculos_cliente ON veiculos (cliente_id);

CREATE TABLE servicos (
    id             UUID PRIMARY KEY,
    nome           TEXT   NOT NULL UNIQUE,
    descricao      TEXT,
    preco_centavos BIGINT NOT NULL CONSTRAINT ck_preco_servico CHECK (preco_centavos >= 0),
    ativo          BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE pecas (
    id                 UUID PRIMARY KEY,
    sku                TEXT    NOT NULL UNIQUE,
    nome               TEXT    NOT NULL,
    tipo               TEXT    NOT NULL CHECK (tipo IN ('PECA', 'INSUMO')),
    quantidade_estoque INTEGER NOT NULL DEFAULT 0 CONSTRAINT ck_estoque_nao_negativo CHECK (quantidade_estoque >= 0),
    estoque_minimo     INTEGER NOT NULL DEFAULT 0 CHECK (estoque_minimo >= 0),
    preco_centavos     BIGINT  NOT NULL CONSTRAINT ck_preco_peca CHECK (preco_centavos >= 0),
    ativo              BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE SEQUENCE ordens_servico_numero_seq START 1001;

CREATE TABLE ordens_servico (
    id                   UUID PRIMARY KEY,
    numero               BIGINT      NOT NULL UNIQUE DEFAULT nextval('ordens_servico_numero_seq'),
    cliente_id           UUID        NOT NULL REFERENCES clientes (id),
    veiculo_id           UUID        NOT NULL REFERENCES veiculos (id),
    status               TEXT        NOT NULL CONSTRAINT ck_status CHECK (status IN (
        'RECEBIDA', 'EM_DIAGNOSTICO', 'AGUARDANDO_APROVACAO',
        'EM_EXECUCAO', 'FINALIZADA', 'ENTREGUE', 'CANCELADA')),
    descricao_problema   TEXT,
    total_centavos       BIGINT      NOT NULL DEFAULT 0 CHECK (total_centavos >= 0),
    versao               INTEGER     NOT NULL DEFAULT 1,
    criada_em            TIMESTAMPTZ NOT NULL DEFAULT now(),
    execucao_iniciada_em TIMESTAMPTZ,
    finalizada_em        TIMESTAMPTZ,
    entregue_em          TIMESTAMPTZ
);
CREATE INDEX ix_os_status_criada ON ordens_servico (status, criada_em);
CREATE INDEX ix_os_cliente ON ordens_servico (cliente_id);
CREATE INDEX ix_os_finalizadas ON ordens_servico (finalizada_em) WHERE finalizada_em IS NOT NULL;

CREATE TABLE os_itens_servico (
    id               UUID PRIMARY KEY,
    ordem_servico_id UUID   NOT NULL REFERENCES ordens_servico (id) ON DELETE CASCADE,
    servico_id       UUID   NOT NULL REFERENCES servicos (id),
    descricao        TEXT   NOT NULL,
    preco_centavos   BIGINT NOT NULL CHECK (preco_centavos >= 0)
);
CREATE INDEX ix_os_itens_servico_os ON os_itens_servico (ordem_servico_id);

CREATE TABLE os_itens_peca (
    id                  UUID PRIMARY KEY,
    ordem_servico_id    UUID    NOT NULL REFERENCES ordens_servico (id) ON DELETE CASCADE,
    peca_id             UUID    NOT NULL REFERENCES pecas (id),
    descricao           TEXT    NOT NULL,
    quantidade          INTEGER NOT NULL CONSTRAINT ck_qtd_item CHECK (quantidade >= 1),
    preco_unit_centavos BIGINT  NOT NULL CHECK (preco_unit_centavos >= 0)
);
CREATE INDEX ix_os_itens_peca_os ON os_itens_peca (ordem_servico_id);

CREATE TABLE os_historico_status (
    id               UUID PRIMARY KEY,
    ordem_servico_id UUID        NOT NULL REFERENCES ordens_servico (id) ON DELETE CASCADE,
    status_de        TEXT,
    status_para      TEXT        NOT NULL,
    usuario_id       UUID REFERENCES usuarios (id),
    origem           TEXT        NOT NULL CHECK (origem IN ('ADMIN', 'CLIENTE', 'SISTEMA')),
    em               TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_os_historico_os ON os_historico_status (ordem_servico_id, em);

CREATE TABLE movimentacoes_estoque (
    id               UUID PRIMARY KEY,
    peca_id          UUID        NOT NULL REFERENCES pecas (id),
    ordem_servico_id UUID REFERENCES ordens_servico (id),
    tipo             TEXT        NOT NULL CHECK (tipo IN ('ENTRADA', 'BAIXA', 'ESTORNO')),
    quantidade       INTEGER     NOT NULL CHECK (quantidade > 0),
    criado_em        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_mov_estoque_peca ON movimentacoes_estoque (peca_id, criado_em);
