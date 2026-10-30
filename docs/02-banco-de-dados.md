# Banco de dados: PostgreSQL

O enunciado pede que a escolha do banco seja justificada.

**Escolha:** PostgreSQL 17.

**Por quê:**

- **Dados relacionais por natureza.** Cliente → veículo → ordem de serviço → itens (serviços e peças) formam um grafo de relacionamentos. Chaves estrangeiras garantem que não existe OS sem cliente nem item sem OS.
- **Transações ACID.** Aprovar um orçamento e baixar peças do estoque precisa acontecer por inteiro ou não acontecer. Sem transação, uma falha no meio deixaria o estoque inconsistente.
- **Concorrência no estoque.** Duas OS pedindo a última peça ao mesmo tempo é resolvido com `SELECT ... FOR UPDATE` ou `UPDATE` condicional, recursos maduros no Postgres.
- **Dinheiro exato.** O tipo `NUMERIC` (ou inteiros em centavos) evita os erros de arredondamento de ponto flutuante.
- **Recursos úteis ao projeto.** `citext` para e-mail sem diferença de caixa, `CHECK` constraints para reforçar regras no banco e `FILTER`/funções de janela para o cálculo do tempo médio de execução.
- **Ecossistema.** Imagem oficial leve no Docker, driver `pgx` mantido e suporte a testcontainers para os testes de integração.

**Alternativas descartadas:**

| Alternativa | Por que não |
|---|---|
| MongoDB | Modelo de documentos não ajuda aqui: os dados são muito relacionados e o estoque exige transações multi-entidade. |
| MySQL | Atenderia, mas o Postgres tem constraints, tipos e recursos de concorrência mais completos. |
| SQLite | Ótimo para protótipos, mas é um arquivo local: não representa um ambiente com `docker-compose` e escrita concorrente. |
