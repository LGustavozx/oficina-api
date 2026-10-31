# oficina-api

API REST de uma oficina mecânica: ordens de serviço, clientes, veículos, serviços e estoque de peças. Feita em Go, com PostgreSQL, para o Tech Challenge (Fase 1) da pós-graduação.

O projeto está em construção. Por enquanto existe a base (config, banco, migrations, health check), os value objects do domínio (CPF/CNPJ, placa, dinheiro) e o CI. As próximas etapas são login com JWT, cadastros, estoque e ordem de serviço.

## Rodando

Precisa só do Docker.

```bash
docker compose up -d --build
curl localhost:8080/health
```

A resposta esperada é `{"status":"ok"}`. Para parar, `docker compose down`; com `-v` o volume do banco também é apagado.

## Desenvolvimento

Precisa de Go 1.26+ e de um Postgres. O mais simples é subir só o banco pelo compose:

```bash
docker compose up -d db
export DATABASE_URL="postgres://oficina:oficina@localhost:5432/oficina?sslmode=disable"
go run ./cmd/api
```

Testes:

```bash
go test ./...
bash scripts/check-coverage.sh   # confere o mínimo de 80% nos pacotes críticos
```

No CI também rodam `golangci-lint`, `go vet`, `go test -race` e `govulncheck`.

## Configuração

| Variável | Padrão | |
|---|---|---|
| `DATABASE_URL` | — | obrigatória |
| `HTTP_PORT` | `8080` | |

## Estrutura

```
cmd/api/              ponto de entrada
internal/platform/    config, banco e servidor HTTP
internal/shared/      erros, dinheiro e relógio, usados por todos os contextos
internal/registry/    cadastros (CPF/CNPJ e placa por enquanto)
docs/                 decisões e documentação
```

Mais detalhes em [docs/](docs/README.md).
