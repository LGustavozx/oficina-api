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

## Erros

O domínio e a aplicação devolvem `apperr.Error`, com categoria (`Kind`), código estável em inglês e mensagem em português. Só a camada HTTP converte a categoria em status: validação → 400, não encontrado → 404, conflito → 409, não autenticado → 401. Qualquer outro erro vira 500 com mensagem genérica, e o detalhe vai só para o log.

## Autenticação

- Senhas são guardadas como hash bcrypt.
- O login devolve um JWT assinado com HS256, com emissor, expiração e o ID do usuário. A verificação aceita só HS256 e exige expiração.
- Rotas administrativas passam por `RequireAuth`, que valida o token e confirma que o usuário ainda existe.
- E-mail desconhecido e senha errada devolvem o mesmo erro, e a comparação de hash roda nos dois casos, para o tempo de resposta não revelar quais e-mails existem.
