# Sistema Integrado de Atendimento e Execução de Serviços — Oficina Mecânica

Projeto acadêmico — SOAT (Software Architecture), Fase 1 (Tech Challenge). MVP do back-end de uma oficina mecânica de médio porte.

> **Status:** Etapa 1 — documentação. O código será desenvolvido nas etapas seguintes, guiado por esta documentação. Seções marcadas com *(planejado)* dependem da implementação.

## 1. Objetivo

Substituir o controle manual (anotações e planilhas) da oficina por um sistema que organiza ordens de serviço (OS), clientes, veículos, serviços e estoque de peças, permitindo que o cliente acompanhe e aprove o serviço via API. Detalhes em [Visão geral](docs/01-visao-geral.md).

## 2. Descrição resumida da solução

API REST em Go que cobre: abertura de OS com orçamento automático, aprovação pelo cliente, acompanhamento por status (Recebida → Em diagnóstico → Aguardando aprovação → Em execução → Finalizada → Entregue), CRUD administrativo com controle de estoque e métrica de tempo médio de execução. Rotas administrativas protegidas por JWT.

## 3. Arquitetura

Monolito modular em **camadas** (apresentação, aplicação, domínio, infraestrutura), com **DDD** (contextos: Ordem de Serviço, Cadastro, Catálogo, Estoque, Identidade). Ver [Arquitetura](docs/04-arquitetura.md) e [Modelagem DDD](docs/03-ddd.md).

## 4. Tecnologias

| Item | Escolha |
|------|---------|
| Linguagem | Go 1.26 |
| Roteamento HTTP | chi |
| Banco de dados | PostgreSQL 16 |
| Acesso a dados | pgx + sqlc |
| Migrations | golang-migrate |
| Autenticação | JWT (HS256) |
| Documentação da API | OpenAPI/Swagger (swaggo) |
| Testes | testing, testify, testcontainers-go |
| Análise de vulnerabilidades | govulncheck, gosec |
| Contêineres | Docker e Docker Compose |

## 5. Pré-requisitos

- Docker e Docker Compose
- Go 1.26 (apenas para rodar testes fora do contêiner)

## 6. Execução *(planejado)*

```bash
cp .env.example .env      # ajuste os valores
docker compose up --build
```

API em `http://localhost:8080/api/v1`. Mais em [Infraestrutura](docs/09-infraestrutura.md).

## 7. Testes *(planejado)*

```bash
go test ./... -race -cover                       # unitários
go test ./... -tags=integration                  # integração (requer Docker)
go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out
```

Meta: cobertura ≥ 80% nos domínios críticos. Ver [Qualidade e testes](docs/08-qualidade-testes.md).

## 8. Documentação da API (Swagger)

`http://localhost:8080/swagger/index.html` *(planejado)*. Catálogo de endpoints em [API REST](docs/06-api.md).

## 9. Banco de dados: decisão e justificativa

**PostgreSQL 16**, por consistência transacional (OS + estoque), dados fortemente relacionais, restrições declarativas (`UNIQUE`, `CHECK`) e agregações nativas para o tempo médio. Justificativa completa e alternativas em [Modelo de dados](docs/05-modelo-de-dados.md#51-banco-escolhido-postgresql-16) e [ADR-003](docs/10-adrs.md).

## 10. Autenticação e credenciais de demonstração

- Login em `POST /api/v1/auth/login` retorna um JWT; enviar como `Authorization: Bearer <token>`.
- Usuário administrador criado por *seed* a partir de `ADMIN_EMAIL` e `ADMIN_PASSWORD` do `.env`. Valores de demonstração no `.env.example` (não usar em produção).
- Cliente consulta/aprova OS sem login, com número da OS + CPF/CNPJ (`X-Documento`).

Detalhes em [Segurança](docs/07-seguranca.md).

## 11. Estrutura dos principais diretórios *(planejado)*

```
cmd/api/            ponto de entrada e composição
internal/
  ordemservico/     contexto core
  cadastro/         clientes e veículos
  catalogo/         serviços
  estoque/          peças e insumos
  identidade/       usuários e JWT
  shared/           erros, dinheiro, relógio, logger
migrations/         SQL versionado
docs/               documentação (DDD, arquitetura, API, segurança, ADRs)
```

## 12. Documentação DDD

[docs/03-ddd.md](docs/03-ddd.md) — linguagem ubíqua, subdomínios, contextos, agregados, eventos e máquina de estados.

## 13. Vídeo da entrega

`<link do vídeo — a preencher>`

## 14. Análise de vulnerabilidades

Relatório em [docs/relatorios/vulnerabilidades.md](docs/relatorios/vulnerabilidades.md) *(a gerar com govulncheck e gosec)*. Processo em [Segurança](docs/07-seguranca.md#79-análise-de-vulnerabilidades).

## Índice completo da documentação

| # | Documento | Conteúdo |
|---|-----------|----------|
| 1 | [Visão geral](docs/01-visao-geral.md) | Contexto, problema, escopo e premissas |
| 2 | [Requisitos](docs/02-requisitos.md) | RF, RNF e regras de negócio |
| 3 | [Modelagem DDD](docs/03-ddd.md) | Linguagem ubíqua, contextos, agregados, estados |
| 4 | [Arquitetura](docs/04-arquitetura.md) | Camadas, pastas, fluxos |
| 5 | [Modelo de dados](docs/05-modelo-de-dados.md) | ER e justificativa do banco |
| 6 | [API REST](docs/06-api.md) | Endpoints e contratos |
| 7 | [Segurança](docs/07-seguranca.md) | JWT, validações, LGPD, vulnerabilidades |
| 8 | [Qualidade e testes](docs/08-qualidade-testes.md) | Estratégia e cobertura |
| 9 | [Infraestrutura](docs/09-infraestrutura.md) | Docker e execução local |
| 10 | [ADRs](docs/10-adrs.md) | Decisões arquiteturais |
| 11 | [Entrega da Fase 1](docs/11-entrega-fase1.md) | Checklist, capa do PDF, fluxo de PR |

## Convenções

- **[Premissa]** marca decisões assumidas na ausência de definição no enunciado, a validar.
- Diagramas em [Mermaid](https://mermaid.js.org/).
