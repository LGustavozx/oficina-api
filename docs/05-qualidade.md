# Qualidade

Números medidos em 31/10/2026, ao final do desenvolvimento. Para reproduzir:

```bash
export TEST_DATABASE_URL="postgres://oficina:oficina@localhost:5432/oficina?sslmode=disable"   # com o banco do compose no ar
bash scripts/quality-report.sh    # gera reports/ com tudo o que aparece abaixo
```

No CI, o job **Relatórios de qualidade** roda o mesmo script e publica a pasta `reports/` como artefato (`relatorios-de-qualidade`), com a cobertura em HTML, o resumo por função e o relatório do `govulncheck`.

## Testes

| | |
|---|---|
| Funções de teste | 256 |
| Casos (com subtestes) | 784 |
| Falhas | 0 |
| Código de produção / código de teste | cerca de 5.000 / 8.500 linhas |

Os testes de integração usam um PostgreSQL de verdade (`TEST_DATABASE_URL`); no CI ele sobe como serviço do job. Sem a variável eles são pulados, e o `go test ./...` comum continua funcionando sem banco.

O que os testes de integração cobrem, entre outras coisas:

- o **fluxo principal completo**, por HTTP: cadastro de cliente, veículo, serviço e peça, abertura da OS, diagnóstico, orçamento, aprovação com baixa de estoque, finalização e entrega;
- a **aprovação "tudo ou nada"**: sem saldo em qualquer peça, nada muda, e a baixa da primeira peça é desfeita;
- **concorrência**: 20 saídas simultâneas de 1 unidade sobre um estoque de 10 dão exatamente 10 sucessos e 10 recusas; aprovações simultâneas da mesma OS consomem o estoque uma única vez; migrations simultâneas não se atropelam;
- o **acesso do cliente**: só vê e age na própria OS, e toda falha de identificação dá a mesma resposta;
- a **integridade**: cliente, veículo, serviço e peça usados numa OS não podem ser apagados.

## Cobertura

Cobertura total de **92,8%** das instruções. O script `scripts/check-coverage.sh` exige no mínimo **80%** em cada um dos 28 pacotes críticos (domínio, aplicação, infraestrutura e HTTP de cada contexto, além de `platform` e `shared`), e roda no CI. Os menores valores são:

| Pacote | Cobertura |
|---|---|
| `catalog/infrastructure` | 82,9% |
| `registry/infrastructure` | 84,4% |
| `workorder/infrastructure` | 86,5% |
| `platform/database` | 87,0% |
| `identity/infrastructure` | 88,9% |

Todos os pacotes de `domain`, `application` e `http` ficam entre 97,9% e 100%. Ficam fora da lista de críticos o `cmd/api` (só liga o processo) e o `testdb` (ajuda dos testes); o `internal/app` também não entra, mas é exercitado pelos testes de integração acima.

## Análise estática

O CI barra o merge se qualquer um falhar: `gofmt`, `go vet` e `golangci-lint` (com `gosec` para segurança, `errorlint` e `bodyclose`), além do `go test -race`, que detecta acessos concorrentes inseguros. As regras de proteção do GitHub exigem os três jobs de verificação antes de qualquer merge em `develop` ou `main`.

## Vulnerabilidades

Verificadas com o `govulncheck`, que só aponta o que o código realmente chama:

- **Código afetado: 0 vulnerabilidades.**
- Há um aviso informativo sobre o pacote `golang.org/x/crypto/openpgp`, sem correção disponível (GO-2026-5932). O projeto usa só o `bcrypt` desse módulo, e o próprio relatório confirma que nenhum código nosso chama o pacote afetado.

No começo do desenvolvimento o `govulncheck` apontou 15 vulnerabilidades na biblioteca padrão do Go 1.26.3; elas foram resolvidas subindo o `go.mod` para a versão corrigida da linguagem. É por isso que o scan roda no CI: a lista de vulnerabilidades conhecidas muda com o tempo.

## Segurança, em resumo

- Senhas guardadas com bcrypt; o login devolve a mesma resposta para e-mail inexistente e senha errada, e compara o hash nos dois casos.
- JWT assinado com HS256, que aceita só esse algoritmo e exige emissor e expiração (testado contra `alg: none`, outro algoritmo, token adulterado e expirado).
- Segredos só por variável de ambiente; o `JWT_SECRET` precisa de 32 caracteres no mínimo.
- Rotas públicas protegidas por número da OS + documento do cliente, sem vazar a existência de OS de outros clientes.
- A API nunca devolve detalhes internos: erros inesperados viram um `500` genérico, e o detalhe vai só para o log. O log de requisições não registra corpo, cabeçalhos nem parâmetros de URL.
- Entradas validadas no domínio: CPF/CNPJ, placa, e-mail, telefone, textos (inclusive bytes que não são UTF-8) e valores monetários.
