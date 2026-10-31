# oficina-api

API REST de uma oficina mecânica: ordens de serviço, clientes, veículos, serviços e estoque de peças. Feita em Go, com PostgreSQL, para o Tech Challenge (Fase 1) da pós-graduação.

O projeto está em construção. Já existem a base (config, banco, migrations, health check), os value objects do domínio (CPF/CNPJ, placa, dinheiro), o login com JWT, os cadastros (clientes, veículos, serviços e peças com estoque), a ordem de serviço com orçamento, o acompanhamento pelo cliente, o tempo médio de execução e o CI. A próxima etapa é a documentação Swagger.

## Rodando

Precisa só do Docker.

```bash
docker compose up -d --build
curl localhost:8080/health
```

A resposta esperada é `{"status":"ok"}`. Para parar, `docker compose down`; com `-v` o volume do banco também é apagado.

Na primeira execução é criado um administrador com os valores de `ADMIN_EMAIL` e `ADMIN_PASSWORD` (no compose: `admin@oficina.local` / `troque-esta-senha`). Esses padrões servem só para desenvolvimento local.

## Usando a API

```bash
# login: devolve um token JWT
curl -X POST localhost:8080/auth/login \
  -d '{"email":"admin@oficina.local","password":"troque-esta-senha"}'

# rota protegida: usa o token recebido
curl localhost:8080/admin/me -H "Authorization: Bearer <token>"
```

| Rota | Acesso | |
|---|---|---|
| `GET /health` | público | estado da API e do banco |
| `POST /auth/login` | público | `{"email","password"}` → `{"token","expires_at"}` |
| `GET /admin/me` | JWT | dados do usuário autenticado |
| `POST /admin/customers` | JWT | cria cliente (`name`, `document`, `email`, `phone`) |
| `GET /admin/customers` | JWT | lista paginada (`?limit=&offset=`) ou busca por `?document=` |
| `GET /admin/customers/{id}` | JWT | detalhe |
| `PUT /admin/customers/{id}` | JWT | substitui os dados |
| `DELETE /admin/customers/{id}` | JWT | remove |

`document` aceita CPF ou CNPJ, com ou sem pontuação. `email` e `phone` são opcionais.

Veículos seguem o mesmo formato em `/admin/vehicles` (POST, GET, GET `/{id}`, PUT `/{id}`, DELETE `/{id}`):

```json
{"customer_id": "<id do cliente>", "plate": "ABC-1D23", "brand": "Fiat", "model": "Uno", "year": 2010}
```

Serviços do catálogo ficam em `/admin/services`, com as mesmas 5 rotas. O preço é enviado em **centavos**:

```json
{"name": "Troca de óleo", "description": "Óleo e filtro", "price_cents": 12000, "estimated_minutes": 45}
```

O nome do serviço é único (sem diferenciar maiúsculas). Preço e duração são obrigatórios e maiores que zero.

Peças ficam em `/admin/parts` (mesmas 5 rotas, com busca por `?sku=`). O cadastro não define o saldo: o estoque só muda por movimentação.

```json
{"sku": "FLT-OLEO-01", "name": "Filtro de óleo", "description": "Motor 1.0", "unit_price_cents": 3590}
```

| Rota | |
|---|---|
| `POST /admin/parts/{id}/stock-movements` | `{"type": "IN" ou "OUT", "quantity": 10, "reason": "Compra NF 123"}`; devolve a movimentação e o `stock_quantity` atual |
| `GET /admin/parts/{id}/stock-movements` | histórico, da mais recente para a mais antiga |

Uma saída maior que o saldo é recusada com `409 INSUFFICIENT_STOCK`, mesmo com várias requisições simultâneas. O saldo máximo por peça é 1.000.000. Uma peça com histórico de movimentações não pode ser apagada.

### Ordem de serviço

```bash
# abrir a OS: o cliente é deduzido do veículo; serviços e peças são opcionais
POST /admin/work-orders
{"vehicle_id": "<id>", "description": "Barulho no motor",
 "services": ["<id do serviço>"], "parts": [{"part_id": "<id>", "quantity": 2}]}
```

O orçamento é calculado sozinho: `total_cents` é a soma de preço × quantidade dos itens, e cada item guarda o preço do momento (reajustar o catálogo depois não altera uma OS existente). A OS nasce `RECEIVED` e avança por ações explícitas:

| Ação | De | Para |
|---|---|---|
| `POST /admin/work-orders/{id}/start-diagnosis` | `RECEIVED` | `IN_DIAGNOSIS` |
| `PUT  /admin/work-orders/{id}/items` | `RECEIVED` ou `IN_DIAGNOSIS` | troca os serviços e peças |
| `POST /admin/work-orders/{id}/send-quote` | `IN_DIAGNOSIS` | `AWAITING_APPROVAL` (exige ao menos um item) |
| `POST /admin/work-orders/{id}/approve` | `AWAITING_APPROVAL` | `IN_EXECUTION` |
| `POST /admin/work-orders/{id}/reject` | `AWAITING_APPROVAL` | `CANCELLED` |
| `POST /admin/work-orders/{id}/complete` | `IN_EXECUTION` | `COMPLETED` |
| `POST /admin/work-orders/{id}/deliver` | `COMPLETED` | `DELIVERED` |

Uma ação fora do status certo devolve `409 INVALID_STATUS_TRANSITION`. Também há `GET /admin/work-orders` (lista, com `?status=` e paginação) e `GET /admin/work-orders/{id}`.

**Estoque:** a baixa das peças acontece na aprovação, na mesma transação que muda o status. Se qualquer peça não tiver saldo, a aprovação inteira é recusada (`409 INSUFFICIENT_STOCK`) e nada muda. Recusar o orçamento não mexe no estoque. Clientes, veículos, serviços e peças usados em uma OS não podem ser apagados.

### Acompanhamento pelo cliente (sem login)

O cliente se identifica pelo **número da OS** e pelo **próprio CPF/CNPJ**, enviado no cabeçalho `X-Customer-Document` (no cabeçalho, e não na URL, para não ficar em históricos e logs):

```bash
curl localhost:8080/public/work-orders/100 -H "X-Customer-Document: 529.982.247-25"
curl -X POST localhost:8080/public/work-orders/100/approve -H "X-Customer-Document: 52998224725"
```

| Rota | |
|---|---|
| `GET /public/work-orders/{número}` | status, descrição, itens, total e datas |
| `POST /public/work-orders/{número}/approve` | aprova o orçamento (mesmas regras e mesma baixa de estoque da API administrativa) |
| `POST /public/work-orders/{número}/reject` | recusa o orçamento |

A resposta traz só o que o cliente precisa: nenhum identificador interno. OS inexistente, documento errado, documento inválido e OS de outro cliente dão **a mesma resposta `404`**, para ninguém descobrir quais ordens existem.

### Tempo médio de execução

`GET /admin/metrics/execution-time` (JWT) devolve `{"completed_orders": 12, "average_execution_minutes": 93.3}`. O tempo é medido da aprovação à finalização. O parâmetro opcional `?from=AAAA-MM-DD&to=AAAA-MM-DD` filtra pela data de finalização (`from` incluso, `to` exclusivo).

### Limitações conhecidas

- Não há limite de tentativas (*rate limiting*) nas rotas públicas nem no login; em produção isso ficaria no proxy ou no gateway.
- Os tokens JWT não podem ser revogados antes de expirar.
- Não há envio real de notificação ao cliente: "enviar o orçamento" só muda o status, e o cliente consulta pela API.

Na lista de veículos, `?plate=` faz a busca exata e `?customer_id=` (veículos de um cliente). A placa pode ser no formato antigo ou Mercosul. Um cliente que tem veículos não pode ser apagado (`409 CUSTOMER_IN_USE`).

Erros seguem o formato `{"code":"INVALID_CREDENTIALS","message":"..."}`.

## Desenvolvimento

Precisa de Go 1.26+ e de um Postgres. O mais simples é subir só o banco pelo compose:

```bash
docker compose up -d db
export DATABASE_URL="postgres://oficina:oficina@localhost:5432/oficina?sslmode=disable"
export JWT_SECRET="um-segredo-qualquer-com-pelo-menos-32-caracteres"
go run ./cmd/api
```

Testes:

```bash
go test ./...

# inclui os testes de integração com o banco
TEST_DATABASE_URL="postgres://oficina:oficina@localhost:5432/oficina?sslmode=disable" go test ./...

TEST_DATABASE_URL="postgres://oficina:oficina@localhost:5432/oficina?sslmode=disable" \
  bash scripts/check-coverage.sh   # mínimo de 80% nos pacotes críticos; precisa do banco
```

Sem `TEST_DATABASE_URL`, os testes de integração são pulados. No CI eles rodam com um Postgres de verdade, junto com `golangci-lint`, `go vet`, `go test -race` e `govulncheck`.

## Configuração

| Variável | Padrão | |
|---|---|---|
| `DATABASE_URL` | — | obrigatória |
| `JWT_SECRET` | — | obrigatória, mínimo de 32 caracteres |
| `JWT_TTL` | `1h` | validade do token (`30m`, `2h`...) |
| `HTTP_PORT` | `8080` | |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | — | criam o primeiro administrador se não houver usuários; informar os dois |

## Estrutura

```
cmd/api/              ponto de entrada
internal/platform/    config, banco e servidor HTTP
internal/shared/      erros, dinheiro e relógio, usados por todos os contextos
internal/identity/    login e autenticação
internal/registry/    cadastros: clientes e veículos
internal/catalog/     catálogo de serviços
internal/inventory/   peças e controle de estoque
internal/workorder/   ordem de serviço, orçamento e status
internal/app/         montagem: liga os contextos entre si
docs/                 decisões e documentação
```

Mais detalhes em [docs/](docs/README.md).
