# Requisitos do desafio e onde foram atendidos

Tabela do enunciado do Tech Challenge (Fase 1) contra o que foi entregue.

## Funcionalidades obrigatórias

### Criação da Ordem de Serviço

| Requisito | Atendimento |
|---|---|
| Identificação do cliente por CPF/CNPJ | `GET /admin/customers?document=` busca pelo documento; CPF e CNPJ (inclusive o alfanumérico) são validados pelos dígitos verificadores. A OS usa o cliente dono do veículo. |
| Cadastro de veículo (placa, marca, modelo, ano) | `/admin/vehicles`; placa no formato antigo e Mercosul, ano validado. |
| Inclusão dos serviços solicitados | `services` na criação da OS, ou `PUT /admin/work-orders/{id}/items`. |
| Peças e insumos necessários | `parts` (com quantidade), nos mesmos pontos. |
| Orçamento gerado automaticamente | `total_cents` é sempre calculado a partir dos itens; cada item guarda o preço do momento. |
| Envio do orçamento ao cliente para aprovação | `POST /admin/work-orders/{id}/send-quote` leva a OS para `AWAITING_APPROVAL`; o cliente aprova ou recusa em `/public/work-orders/{número}/approve` e `/reject`. |

### Acompanhamento da OS

| Requisito | Atendimento |
|---|---|
| Status: Recebida, Em diagnóstico, Aguardando aprovação, Em execução, Finalizada, Entregue | `RECEIVED`, `IN_DIAGNOSIS`, `AWAITING_APPROVAL`, `IN_EXECUTION`, `COMPLETED`, `DELIVERED`. Foi acrescentado `CANCELLED`, para o caso de o cliente recusar o orçamento. |
| Alteração automática dos status conforme ações no sistema | Cada ação (`start-diagnosis`, `send-quote`, `approve`, `reject`, `complete`, `deliver`) move a OS para o status seguinte e só vale a partir do status certo. |
| Consulta pelo cliente via API | `GET /public/work-orders/{número}`, com o número da OS e o CPF/CNPJ do cliente. |

### Gestão administrativa

| Requisito | Atendimento |
|---|---|
| CRUD de clientes | `/admin/customers` |
| CRUD de veículos | `/admin/vehicles` |
| CRUD de serviços | `/admin/services` |
| CRUD de peças e insumos, com controle de estoque | `/admin/parts`, mais `/admin/parts/{id}/stock-movements` (entrada, saída e histórico). O saldo só muda por movimentação, e a saída é atômica. |
| Listagem e detalhamento de ordens de serviço | `GET /admin/work-orders` (filtro por status, paginação) e `GET /admin/work-orders/{id}` |
| Monitoramento do tempo médio de execução | `GET /admin/metrics/execution-time`, com período opcional |

### Segurança e qualidade

| Requisito | Atendimento |
|---|---|
| Autenticação JWT nas APIs administrativas | `POST /auth/login`; todas as rotas `/admin/*` exigem o token. Senhas com bcrypt. |
| Validação de dados sensíveis (CPF/CNPJ, placa) | Value objects `Document` e `Plate`, que só existem válidos. |
| Testes unitários e de integração nos principais fluxos | 256 funções de teste (784 casos). O fluxo completo, do cadastro à entrega, é testado por HTTP contra um Postgres de verdade. Ver [05-qualidade.md](05-qualidade.md). |

## Requisitos técnicos

| Requisito | Atendimento |
|---|---|
| Back-end monolítico em camadas, com DDD | Monolito modular; cada contexto tem `domain`, `application`, `infrastructure` e `http`. Ver [03-ddd.md](03-ddd.md) e [04-arquitetura.md](04-arquitetura.md). |
| Banco de dados livre, com justificativa | PostgreSQL. Ver [02-banco-de-dados.md](02-banco-de-dados.md). |
| APIs RESTful documentadas via Swagger | `openapi.yaml` escrito à mão, servido em `/docs` (Swagger UI) e `/openapi.yaml`. Um teste impede que ele fique diferente das rotas do código. |
| Dockerfile para build da aplicação | [Dockerfile](../Dockerfile): build em dois estágios, imagem final sem shell e sem root. |
| docker-compose.yml para o ambiente completo | [docker-compose.yml](../docker-compose.yml): API e PostgreSQL, com health check. |
| Testes automatizados, com cobertura mínima de 80% nos domínios críticos | O script `scripts/check-coverage.sh` falha abaixo de 80% e roda no CI. Todos os 28 pacotes críticos estão acima disso; a cobertura total é de 92,8%. |
| Execução local simples (README.md) | [README.md](../README.md) |

## Decisões que o enunciado deixou em aberto

- **Quando baixar o estoque:** na aprovação do orçamento, na mesma transação que muda o status. Se faltar qualquer peça, a aprovação inteira é recusada.
- **Como o cliente se identifica sem conta:** número da OS + CPF/CNPJ em cabeçalho. Qualquer falha de identificação dá a mesma resposta `404`.
- **Orçamento recusado:** a OS vai para `CANCELLED`; como o estoque só baixa na aprovação, não há nada a devolver.
- **Dinheiro:** inteiros em centavos, em toda a API.
