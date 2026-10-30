# oficina-api — instruções para o Claude

Projeto acadêmico individual (SOAT, Tech Challenge Fase 1): monolito em Go, API REST de uma oficina mecânica. Documentação em `docs/` (começar por `README.md`).

## Convenções

- **Identificadores em inglês** (tipos, funções, variáveis, campos, constantes, pacotes). **Comentários em português.**
- Contratos externos também em inglês: rotas, campos JSON, códigos de erro, chaves de log. Pendente de aplicar onde ainda houver português (ver estado abaixo).
- Banco de dados (tabelas, colunas, valores de status) e mensagens ao usuário permanecem em português.
- Linguagem ubíqua do DDD é em português na documentação, com tabela de correspondência em `docs/03-ddd.md`.
- **Não** incluir `Co-Authored-By` nem linha de atribuição ao Claude em commits ou PRs.
- Os commits são feitos manualmente pelo usuário: deixar as mudanças no disco e sugerir a mensagem; não rodar `git commit` nem `git push` sem pedido explícito.

## Arquitetura

Monolito modular em camadas: `domain` → `application` → `infrastructure` / `http`, por contexto em `internal/` (`identity`, `registry`, e futuros `catalog`, `inventory`, `workorder`). Infra transversal em `internal/platform`, base em `internal/shared`. Detalhes em `docs/04-arquitetura.md`.

## Qualidade (o CI exige)

- `gofmt`, `golangci-lint run ./...`, `go vet ./...`
- `go test ./...` e cobertura ≥ 80% nos pacotes críticos (`scripts/check-coverage.sh`; acrescentar novos pacotes à lista a cada fase)
- `govulncheck ./...` sem vulnerabilidades alcançáveis
- No Windows sem compilador C, `go test -race` não roda localmente; ele roda no CI

## Fluxo de Git

`feature/*` → PR para `develop` → PR `develop` → `main` nos marcos. `main` e `develop` protegidas; nunca contornar a proteção. Conferir que a base do PR é `develop`.

## Estado das fases

- O projeto foi recomeçado do zero (como estudo guiado: cada parte é explicada antes/depois de escrita). Contratos externos já nascem em inglês.
- Fase 0 (config, Postgres, migrations com goose, `/health`, Dockerfile, compose, CI): concluída.
- Próximas: fase 1 (apperr, money, clock, `registry` Document/Plate), fase 2 (identity: login, JWT, `/admin/me`), fase 3 (cadastros, catálogo, estoque + testes de integração com testcontainers), fase 4 (ordem de serviço), fase 5 (rotas públicas e métrica de tempo médio), fase 6 (Swagger, cobertura, relatório de vulnerabilidades, README final).

## Dicas de ambiente (Windows / PowerShell 5.1)

- Enviar JSON ao `curl.exe` por arquivo (`--data @arquivo.json`); o PowerShell remove aspas duplas de strings inline.
- `Set-Content -Encoding UTF8` grava BOM e quebra o `gofmt`; usar `[IO.File]::WriteAllText` com `UTF8Encoding($false)`.
