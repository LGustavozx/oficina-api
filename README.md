# oficina-api

API REST de uma oficina mecânica: clientes, veículos, serviços, peças com controle de estoque e ordens de serviço com orçamento, aprovação e acompanhamento pelo cliente. Feita em Go, com PostgreSQL, para o Tech Challenge (Fase 1) da pós-graduação.

## Rodando

Precisa só do Docker.

```bash
docker compose up -d --build
curl localhost:8080/health      # {"status":"ok"}
```

Para parar, `docker compose down` (com `-v`, o volume do banco também é apagado).

Na primeira execução é criado um administrador com `ADMIN_EMAIL` e `ADMIN_PASSWORD` (no compose: `admin@oficina.local` / `troque-esta-senha`). Esses valores servem só para desenvolvimento local.

## Documentação da API

Com a API rodando, abra **http://localhost:8080/docs**: é o Swagger UI, com todas as rotas, os formatos de entrada e saída e os códigos de erro. A especificação em si está em [internal/docs/openapi.yaml](internal/docs/openapi.yaml) e também é servida em `/openapi.yaml`. Um teste compara as rotas do código com as do arquivo, então a documentação não fica para trás.

A página carrega o Swagger UI de uma CDN (versão fixa e verificada por SRI), então precisa de internet no navegador.

## Um exemplo do fluxo

```bash
# 1. login: devolve um token JWT
curl -X POST localhost:8080/auth/login \
  -d '{"email":"admin@oficina.local","password":"troque-esta-senha"}'

# 2. cadastros (as rotas /admin/* usam o token)
H="Authorization: Bearer <token>"
curl -H "$H" -X POST localhost:8080/admin/customers -d '{"name":"Maria Souza","document":"529.982.247-25"}'
curl -H "$H" -X POST localhost:8080/admin/vehicles  -d '{"customer_id":"<id>","plate":"ABC1D23","brand":"Fiat","model":"Uno","year":2010}'
curl -H "$H" -X POST localhost:8080/admin/services  -d '{"name":"Troca de óleo","price_cents":12000,"estimated_minutes":45}'
curl -H "$H" -X POST localhost:8080/admin/parts     -d '{"sku":"FLT-01","name":"Filtro de óleo","unit_price_cents":3590}'
curl -H "$H" -X POST localhost:8080/admin/parts/<id>/stock-movements -d '{"type":"IN","quantity":10,"reason":"Compra"}'

# 3. abrir a ordem de serviço: o cliente é o dono do veículo, o orçamento é calculado sozinho
curl -H "$H" -X POST localhost:8080/admin/work-orders \
  -d '{"vehicle_id":"<id>","description":"Barulho no motor","services":["<id>"],"parts":[{"part_id":"<id>","quantity":3}]}'

# 4. o atendente avança a OS; o cliente aprova sozinho, só com o número da OS e o seu CPF
curl -H "$H" -X POST localhost:8080/admin/work-orders/<id>/start-diagnosis
curl -H "$H" -X POST localhost:8080/admin/work-orders/<id>/send-quote
curl -X POST localhost:8080/public/work-orders/<número>/approve -H "X-Customer-Document: 529.982.247-25"
curl -H "$H" -X POST localhost:8080/admin/work-orders/<id>/complete
curl -H "$H" -X POST localhost:8080/admin/work-orders/<id>/deliver
```

O status da OS segue `RECEIVED` → `IN_DIAGNOSIS` → `AWAITING_APPROVAL` → `IN_EXECUTION` → `COMPLETED` → `DELIVERED` (ou `CANCELLED`, se o orçamento for recusado). A baixa do estoque acontece na aprovação, na mesma transação que muda o status.

## Desenvolvimento

Precisa de Go 1.26+ e de um Postgres. O mais simples é subir só o banco pelo compose:

```bash
docker compose up -d db
export DATABASE_URL="postgres://oficina:oficina@localhost:5432/oficina?sslmode=disable"
export JWT_SECRET="um-segredo-qualquer-com-pelo-menos-32-caracteres"
go run ./cmd/api
```

## Testes e qualidade

```bash
go test ./...                    # testes unitários; os de integração são pulados sem banco

# com os testes de integração (Postgres de verdade)
export TEST_DATABASE_URL="postgres://oficina:oficina@localhost:5432/oficina?sslmode=disable"
go test ./...
bash scripts/check-coverage.sh   # exige 80% nos pacotes críticos
bash scripts/quality-report.sh   # gera reports/: cobertura (HTML) e vulnerabilidades
```

O CI roda `gofmt`, `go vet`, `golangci-lint`, `go test -race`, a cobertura mínima, o `govulncheck` e publica os relatórios como artefato. Os números atuais e como reproduzi-los estão em [docs/05-qualidade.md](docs/05-qualidade.md).

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
internal/platform/    config, banco, transações e servidor HTTP
internal/shared/      erros, dinheiro, relógio e validação de texto
internal/identity/    login e autenticação (JWT)
internal/registry/    clientes e veículos
internal/catalog/     catálogo de serviços
internal/inventory/   peças e controle de estoque
internal/workorder/   ordem de serviço, orçamento, status e acompanhamento
internal/app/         montagem: liga os contextos entre si
internal/docs/        openapi.yaml e Swagger UI
docs/                 decisões e documentação
```

Cada contexto tem as camadas `domain`, `application`, `infrastructure` e `http`. Mais detalhes em [docs/](docs/README.md).

## Limitações conhecidas

- Não há limite de tentativas (*rate limiting*) no login nem nas rotas públicas; em produção isso ficaria no proxy ou no gateway.
- Os tokens JWT não podem ser revogados antes de expirar.
- Não há envio real de notificação: "enviar o orçamento" só muda o status, e o cliente consulta pela API.
- Os padrões de senha e de segredo do `docker-compose.yml` são públicos e servem só para uso local.
