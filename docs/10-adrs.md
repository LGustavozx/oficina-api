# 10. Registro de decisões arquiteturais (ADRs)

Formato: contexto → decisão → alternativas → consequências. Status: **Proposta** até validação do grupo.

---

## ADR-001 — Monolito modular em camadas

- **Contexto:** MVP com prazo curto, equipe pequena e requisito explícito de back-end monolítico.
- **Decisão:** Um único binário, organizado em módulos por contexto delimitado, cada um com camadas de domínio, aplicação, infraestrutura e apresentação.
- **Alternativas:** Microsserviços (complexidade operacional desnecessária); camadas globais sem módulos (acopla contextos).
- **Consequências:** Deploy simples e transações locais; exige disciplina nas fronteiras entre módulos.

## ADR-002 — DDD tático no contexto core

- **Contexto:** O enunciado exige DDD; o valor do sistema está nas regras da OS.
- **Decisão:** Aplicar agregados, objetos de valor, eventos e linguagem ubíqua. CRUDs de apoio (catálogo, cadastro) mantêm modelo simples, mas com VOs para validação.
- **Alternativas:** DDD completo em todos os contextos (excesso para o MVP); modelo anêmico (regras espalhadas).
- **Consequências:** Regras testáveis isoladamente; mais código de mapeamento.

## ADR-003 — PostgreSQL como banco de dados

- **Contexto:** Necessidade de consistência entre OS e estoque e dados relacionais.
- **Decisão:** PostgreSQL 16.
- **Alternativas:** MySQL, MongoDB, SQLite (ver [5.1](05-modelo-de-dados.md#51-banco-escolhido-postgresql-16)).
- **Consequências:** Forte integridade e agregações simples; requer contêiner de banco no ambiente.

## ADR-004 — Go com `chi`, `pgx` e `sqlc`

- **Contexto:** Preferência por bibliotecas leves e próximas da stdlib, com SQL explícito e tipado.
- **Decisão:** `chi` para roteamento; `pgx` como driver; `sqlc` para gerar código a partir de SQL.
- **Alternativas:** Gin/Echo (mais opinativos); GORM (esconde SQL e favorece modelo anêmico).
- **Consequências:** Menos "mágica" e melhor desempenho; mais SQL escrito manualmente.

## ADR-005 — JWT (HS256) para APIs administrativas

- **Contexto:** Requisito de autenticação JWT; um único emissor e consumidor (monolito).
- **Decisão:** HS256 com segredo por variável de ambiente.
- **Alternativas:** RS256 (útil com múltiplos consumidores); sessões com cookie.
- **Consequências:** Simples e suficiente; sem revogação imediata no MVP (mitigado por expiração curta).

## ADR-006 — Acesso do cliente sem login (CPF/CNPJ + nº da OS)

- **Contexto:** O enunciado prevê consulta do cliente pela API, mas só exige JWT para APIs administrativas.
- **Decisão:** Rotas `/publico` exigem número da OS e documento (cabeçalho), com rate limit e 404 uniforme.
- **Alternativas:** Cadastro e login de clientes (fora do escopo); link com token único por OS (evolução recomendada).
- **Consequências:** Baixo atrito para o cliente; segurança menor que login completo — risco aceito no MVP e documentado.

## ADR-007 — Status adicional "Cancelada"

- **Contexto:** O enunciado lista seis status, mas não define o destino de um orçamento rejeitado.
- **Decisão:** Incluir `Cancelada` como estado terminal.
- **Alternativas:** Manter a OS em "Aguardando aprovação" indefinidamente; voltar a "Em diagnóstico".
- **Consequências:** Fluxo completo e sem estados órfãos; extensão do enunciado a ser validada.

## ADR-008 — Baixa de estoque na aprovação do orçamento

- **Contexto:** É preciso garantir peças para o serviço aprovado sem sofisticar o MVP.
- **Decisão:** Baixa atômica no momento da aprovação (transação única com a mudança de status). Cancelamento posterior estorna.
- **Alternativas:** Reserva na inclusão da peça (bloqueia estoque com OS não aprovadas); baixa apenas ao finalizar (risco de faltar peça).
- **Consequências:** Simples e consistente; itens adicionais pós-aprovação geram nova baixa após nova aprovação.

## ADR-009 — Valores monetários em centavos inteiros

- **Contexto:** Cálculo de orçamento não tolera erros de arredondamento.
- **Decisão:** `int64` em centavos no domínio e `bigint` no banco.
- **Alternativas:** `float64` (impreciso); `decimal` de terceiros (dependência extra).
- **Consequências:** Precisão exata; conversão apenas na apresentação.

## ADR-010 — Notificação simulada por porta

- **Contexto:** O enunciado pede "envio do orçamento", sem definir canal.
- **Decisão:** Interface `Notificador` com implementação de log/registro no MVP.
- **Alternativas:** Integrar SMTP/WhatsApp agora (fora do escopo e dependente de credenciais externas).
- **Consequências:** Fluxo completo testável; troca de canal sem afetar o domínio.

## ADR-011 — Documentação em Markdown + Mermaid

- **Contexto:** Etapa focada em documentação viva, versionada com o código.
- **Decisão:** Markdown em `docs/`, diagramas em Mermaid, OpenAPI gerado por anotações.
- **Alternativas:** Documentos Word/PDF (difícil versionar); ferramentas gráficas externas.
- **Consequências:** Diffs legíveis e renderização nativa em repositórios; exportação para PDF/DOCX possível quando exigida.

---

## Pendências para validação

| # | Item | Referência |
|---|------|------------|
| 1 | Confirmar o status adicional "Cancelada" | ADR-007 |
| 2 | Confirmar acesso do cliente por CPF/CNPJ + nº da OS | ADR-006 |
| 3 | Confirmar momento da baixa de estoque | ADR-008 |
| 4 | Confirmar stack de bibliotecas (chi/pgx/sqlc) | ADR-004 |
| 5 | Definir formato final de entrega (Markdown, PDF, DOCX) e normas (ABNT?) | — |
