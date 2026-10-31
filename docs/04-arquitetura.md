# Arquitetura

Monolito modular em camadas. O código é dividido por **contexto** (`identity`, `registry` e, adiante, `catalog`, `inventory`, `workorder`), e cada contexto tem as suas camadas.

```
internal/
  platform/    infraestrutura transversal: config, banco, servidor HTTP
  shared/      base usada por todos: erros, dinheiro, relógio
  identity/
    domain/          regras e entidades; não depende de nada de fora
    application/     casos de uso; define as interfaces de que precisa
    infrastructure/  PostgreSQL, bcrypt, JWT; implementa essas interfaces
    http/            rotas e handlers
```

## Regra de dependência

```
http ──┐
       ├──> application ──> domain
infrastructure ──┘
```

As setas apontam para dentro: o domínio não conhece banco, HTTP nem JWT. Por isso:

- a regra de negócio é testada sem banco e sem servidor;
- trocar bcrypt por outro algoritmo, ou JWT por outro tipo de token, mexe só em `infrastructure`;
- a camada `application` declara o que precisa (`PasswordHasher`, `TokenManager`) como interface, e a `infrastructure` a implementa.

## Contextos e como conversam

Os contextos são `identity`, `registry` (clientes e veículos), `catalog` (serviços), `inventory` (peças e estoque) e `workorder` (ordem de serviço). Nenhum importa o outro: a ordem de serviço declara, na sua camada `application`, as interfaces de que precisa (consultar um veículo, um serviço ou uma peça, dar baixa no estoque, achar um cliente pelo documento), e o pacote `internal/app` implementa essas interfaces com adaptadores finos sobre os outros contextos. Também é ele que monta tudo e registra as rotas. Os middlewares de autenticação entram nos contextos como parâmetro, e por isso nenhum contexto conhece o `identity`.

## Transações

Aprovar uma ordem de serviço muda o status **e** dá baixa no estoque, e as duas coisas precisam acontecer juntas ou nenhuma. Para isso, `database.RunInTx` abre a transação e a coloca no contexto; os repositórios usam `database.Conn(ctx, db)` e, se há transação no contexto, participam dela. Assim a regra continua na camada `application`, sem que um repositório conheça o outro.

Duas proteções complementares contra concorrência:

- a baixa de estoque é um único `UPDATE ... WHERE quantidade >= N`, com um `CHECK (quantidade >= 0)` na tabela como segunda barreira;
- a mudança de status é um `UPDATE ... WHERE status = <esperado>`, então duas operações simultâneas sobre a mesma OS não se sobrescrevem: só uma vence.

## Documentação da API

O `openapi.yaml` é escrito à mão e embutido no binário (`internal/docs`). Um teste valida o documento, compara as rotas do código com as do arquivo nos dois sentidos e confere que cada operação documentada existe na API, para a documentação não ficar desatualizada.

## Erros

O domínio e a aplicação devolvem `apperr.Error`, com categoria (`Kind`), código estável em inglês e mensagem em português. Só a camada HTTP converte a categoria em status: validação → 400, não encontrado → 404, conflito → 409, não autenticado → 401. Qualquer outro erro vira 500 com mensagem genérica, e o detalhe vai só para o log.

## Autenticação

- Senhas são guardadas como hash bcrypt.
- O login devolve um JWT assinado com HS256, com emissor, expiração e o ID do usuário. A verificação aceita só HS256 e exige expiração.
- Rotas administrativas passam por `RequireAuth`, que valida o token e confirma que o usuário ainda existe.
- E-mail desconhecido e senha errada devolvem o mesmo erro, e a comparação de hash roda nos dois casos, para o tempo de resposta não revelar quais e-mails existem.
