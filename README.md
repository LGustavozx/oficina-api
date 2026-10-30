# oficina-api

Back-end (MVP) de um sistema de oficina mecânica, escrito em Go. Projeto acadêmico — Tech Challenge, Fase 1.

Documentação completa em [docs/](docs/README.md).

## Como rodar

Pré-requisito: Docker.

```bash
docker compose up -d --build
curl localhost:8080/health      # {"status":"ok"}
```

Para parar: `docker compose down` (acrescente `-v` para apagar também o banco).

## Desenvolvimento local

Requer Go 1.26+ e um Postgres acessível.

```bash
docker compose up -d db
export DATABASE_URL="postgres://oficina:oficina@localhost:5432/oficina?sslmode=disable"
go run ./cmd/api
go test ./...
```

## Variáveis de ambiente

| Variável | Obrigatória | Padrão | Descrição |
|---|---|---|---|
| `DATABASE_URL` | sim | — | String de conexão do PostgreSQL |
| `HTTP_PORT` | não | `8080` | Porta HTTP |
