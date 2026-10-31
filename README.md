# oficina-api

API REST de uma oficina mecânica: ordens de serviço, clientes, veículos, serviços e estoque de peças. Feita em Go, com PostgreSQL, para o Tech Challenge (Fase 1) da pós-graduação.

O projeto está em construção. Já existem a base (config, banco, migrations, health check), os value objects do domínio (CPF/CNPJ, placa, dinheiro), o login com JWT, o cadastro de clientes e o CI. As próximas etapas são veículos, serviços, estoque e ordem de serviço.

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

bash scripts/check-coverage.sh   # confere o mínimo de 80% nos pacotes críticos
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
internal/registry/    cadastros: clientes (veículos em seguida)
docs/                 decisões e documentação
```

Mais detalhes em [docs/](docs/README.md).
