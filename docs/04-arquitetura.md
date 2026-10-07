# 4. Arquitetura

## 4.1 Visão geral

Monolito **modular** em **camadas**, implantado como um único binário Go e um banco PostgreSQL.

```mermaid
flowchart TB
    Cliente([Cliente / App]) -->|HTTP| API
    Admin([Atendente / Mecânico]) -->|HTTP + JWT| API
    subgraph Monolito Go
        API[Camada de Apresentação<br/>handlers, middlewares, DTOs]
        APP[Camada de Aplicação<br/>casos de uso]
        DOM[Camada de Domínio<br/>entidades, VOs, regras]
        INF[Camada de Infraestrutura<br/>repositórios, JWT, notificador]
        API --> APP --> DOM
        INF -. implementa portas .-> DOM
        APP --> INF
    end
    INF --> DB[(PostgreSQL)]
```

## 4.2 Camadas e responsabilidades

| Camada | Responsabilidade | Pode depender de |
|--------|------------------|------------------|
| **Apresentação** (`interfaces/http`) | Rotas, validação de entrada, serialização, mapeamento de erros para HTTP, middleware JWT | Aplicação |
| **Aplicação** (`application`) | Casos de uso; orquestra domínio e transações; sem regra de negócio | Domínio |
| **Domínio** (`domain`) | Entidades, VOs, regras, eventos, **interfaces** de repositório e portas | Nenhuma (só stdlib) |
| **Infraestrutura** (`infrastructure`) | Postgres, JWT, bcrypt, notificador, relógio, config | Domínio |

**Regra de dependência:** as setas apontam para dentro. O domínio não conhece HTTP, SQL nem frameworks (inversão de dependência via interfaces).

## 4.3 Estrutura de pastas proposta

```
.
├── cmd/
│   └── api/main.go                 # composição (injeção de dependências)
├── internal/
│   ├── registry/                   # contexto Cadastro: clientes e veículos
│   │   ├── domain/                 #   customer.go, vehicle.go, document.go, plate.go, repository.go
│   │   ├── application/            #   casos de uso
│   │   ├── infrastructure/         #   repositório postgres
│   │   └── http/                   #   handlers e DTOs
│   ├── catalog/                    # contexto Catálogo de Serviços
│   ├── inventory/                  # contexto Estoque: peças e insumos
│   ├── workorder/                  # contexto Ordem de Serviço (core)
│   │   ├── domain/                 #   work_order.go, status.go, quote.go, events.go
│   │   ├── application/            #   create_order, send_quote, approve, finish...
│   │   ├── infrastructure/
│   │   └── http/
│   ├── identity/                   # contexto Identidade e Acesso (implementado)
│   │   ├── domain/                 #   user.go, ports.go
│   │   ├── application/            #   authenticate.go, ensure_admin.go
│   │   ├── infrastructure/         #   security/ (bcrypt, JWT) e postgres/
│   │   └── http/                   #   handler.go (login, /admin/me) e middleware
│   ├── platform/                   # infraestrutura transversal
│   │   ├── database/               #   pool pgx e migrations
│   │   ├── httpserver/             #   roteador, /health, /ready, registro de módulos
│   │   └── httpx/                  #   JSON, decodificação segura, mapeamento de erros
│   └── shared/                     # apperr, money, clock, config
├── migrations/                     # SQL versionado
├── docs/                           # esta documentação + swagger gerado
├── Dockerfile
├── docker-compose.yml
└── .env.example
```

> Cada contexto tem as quatro camadas internas. Contextos só se comunicam por **interfaces** declaradas no consumidor (ex.: `workorder` declara `StockControl`; `inventory` a implementa), montadas em `cmd/api`.

**Convenção de nomes:** identificadores do código (tipos, funções, variáveis, campos, pacotes) são em **inglês**; comentários são em **português**. Os contratos externos permanecem em português: rotas, campos JSON, tabelas/colunas do banco, códigos de erro e mensagens ao usuário. A correspondência com a linguagem ubíqua está em [3.1](03-ddd.md#31-linguagem-ubíqua).

**Registro de módulos:** cada contexto implementa `httpserver.Module` (`Register(public, admin chi.Router)`). Tudo registrado em `admin` fica sob `/api/v1/admin` e passa pelo middleware JWT, inclusive rotas inexistentes (respondem 401, sem revelar a estrutura da API).

## 4.4 Fluxo: abertura de OS e envio de orçamento

```mermaid
sequenceDiagram
    actor A as Atendente
    participant H as Handler HTTP
    participant UC as Caso de Uso
    participant D as OrdemServico (domínio)
    participant R as Repositório
    participant N as Notificador

    A->>H: POST /admin/ordens-servico (JWT)
    H->>UC: CriarOS(cmd)
    UC->>R: buscar cliente e veículo (RN04)
    UC->>D: NovaOS(...) [status Recebida]
    UC->>R: salvar
    H-->>A: 201 Created

    A->>H: POST /admin/ordens-servico/{id}/orcamento
    H->>UC: EnviarOrcamento(id)
    UC->>R: carregar OS
    UC->>D: EnviarOrcamento() [valida RN05, calcula total]
    UC->>R: salvar (status Aguardando aprovação)
    UC->>N: EnviarOrcamento(cliente, os)
    H-->>A: 200 OK
```

## 4.5 Fluxo: aprovação pelo cliente

```mermaid
sequenceDiagram
    actor C as Cliente
    participant H as Handler público
    participant UC as AprovarOrcamento
    participant D as OrdemServico
    participant E as ControleEstoque
    participant DB as Transação

    C->>H: POST /publico/ordens-servico/{numero}/aprovacao {documento}
    H->>UC: Aprovar(numero, documento)
    UC->>DB: inicia transação
    UC->>D: valida documento (RN13) e Aprovar()
    UC->>E: Baixar(peças da OS)
    alt estoque insuficiente
        E-->>UC: ErrEstoqueInsuficiente
        UC->>DB: rollback
        H-->>C: 409 Conflict
    else sucesso
        UC->>DB: commit (OS Em execução + estoque)
        H-->>C: 200 OK
    end
```

## 4.6 Tratamento de erros

| Erro de domínio | HTTP |
|-----------------|------|
| Validação (CPF, placa, valores) | 422 Unprocessable Entity |
| Não encontrado | 404 |
| Transição inválida, estoque insuficiente, duplicidade | 409 Conflict |
| Não autenticado / token inválido | 401 |
| Sem permissão | 403 |
| Inesperado | 500 (sem vazar detalhes) |

Formato padrão (RFC 7807 simplificado):

```json
{ "codigo": "TRANSICAO_INVALIDA", "mensagem": "Não é possível finalizar uma OS que está Recebida", "detalhes": [] }
```

## 4.7 Transações e concorrência

- Cada caso de uso que altera OS e estoque executa em **uma única transação**.
- Baixa de estoque com `UPDATE pecas SET quantidade = quantidade - $1 WHERE id = $2 AND quantidade >= $1` (atômico, evita negativo sob concorrência).
- OS usa **controle otimista** (coluna `versao`) para evitar sobrescrita concorrente.

## 4.8 Observabilidade

- Logs estruturados JSON (`log/slog`) com `request_id`.
- `GET /health` (liveness) e `GET /ready` (checa banco).

## 4.9 Evolução para microsserviços

A separação em contextos com portas explícitas permite extrair, no futuro, `estoque` ou `notificação` como serviços independentes, trocando a implementação da porta por um cliente HTTP/mensageria sem alterar o domínio.
