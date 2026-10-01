# 2. Requisitos

Prioridade: **M** = Must (obrigatório no enunciado), **S** = Should (recomendado).

## 2.1 Requisitos funcionais

### Criação da Ordem de Serviço

| ID | Requisito | Prio. |
|----|-----------|-------|
| RF01 | Identificar o cliente por CPF ou CNPJ ao abrir uma OS | M |
| RF02 | Cadastrar veículo (placa, marca, modelo, ano) vinculado ao cliente | M |
| RF03 | Incluir na OS os serviços solicitados (ex.: troca de óleo, alinhamento) | M |
| RF04 | Incluir na OS peças e insumos necessários | M |
| RF05 | Gerar orçamento automaticamente com base em serviços e peças | M |
| RF06 | Enviar o orçamento ao cliente para aprovação | M |
| RF07 | Cliente aprovar ou rejeitar o orçamento via API | M |

### Acompanhamento da OS

| ID | Requisito | Prio. |
|----|-----------|-------|
| RF08 | Manter status da OS: Recebida, Em diagnóstico, Aguardando aprovação, Em execução, Finalizada, Entregue | M |
| RF09 | Alterar status automaticamente conforme ações no sistema | M |
| RF10 | Cliente consultar o andamento da OS via API | M |
| RF11 | Registrar histórico de transições de status (quem, quando) | S |

### Gestão administrativa

| ID | Requisito | Prio. |
|----|-----------|-------|
| RF12 | CRUD de clientes | M |
| RF13 | CRUD de veículos | M |
| RF14 | CRUD de serviços (catálogo com preço) | M |
| RF15 | CRUD de peças e insumos com controle de estoque | M |
| RF16 | Listar e detalhar ordens de serviço (filtros por status, cliente, período) | M |
| RF17 | Monitorar o tempo médio de execução dos serviços | M |
| RF18 | Registrar movimentações de estoque (entrada, reserva, baixa) | S |

### Segurança

| ID | Requisito | Prio. |
|----|-----------|-------|
| RF19 | Autenticar usuários administrativos e emitir JWT | M |
| RF20 | Proteger todas as rotas administrativas com JWT | M |
| RF21 | Validar CPF/CNPJ (dígitos verificadores) e placa (padrão antigo e Mercosul) | M |

## 2.2 Requisitos não funcionais

| ID | Categoria | Requisito |
|----|-----------|-----------|
| RNF01 | Arquitetura | Back-end monolítico em arquitetura em camadas, com DDD |
| RNF02 | Linguagem | Go |
| RNF03 | Dados | Banco relacional com justificativa documentada ([ADR-003](10-adrs.md)) |
| RNF04 | API | RESTful, documentada via Swagger/OpenAPI |
| RNF05 | Portabilidade | Dockerfile e docker-compose.yml para ambiente completo |
| RNF06 | Qualidade | Cobertura mínima de 80% nos domínios críticos |
| RNF07 | Qualidade | Testes unitários e de integração nos fluxos principais |
| RNF08 | Usabilidade do projeto | README com execução local simples |
| RNF09 | Segurança | Senhas com hash (bcrypt), segredos por variável de ambiente, LGPD para dados pessoais |
| RNF10 | Desempenho **[Premissa]** | Consultas comuns respondem em menos de 300 ms com 10 mil OS |
| RNF11 | Observabilidade | Logs estruturados (JSON) e endpoint de health check |
| RNF12 | Consistência | Operações de OS e estoque em transação atômica |

## 2.3 Regras de negócio

| ID | Regra |
|----|-------|
| RN01 | CPF/CNPJ deve ser válido e único por cliente |
| RN02 | Placa deve ser válida (`AAA-9999` ou Mercosul `AAA9A99`) e única por veículo |
| RN03 | Ano do veículo entre 1950 e ano corrente + 1 |
| RN04 | O veículo da OS deve pertencer ao cliente da OS |
| RN05 | Uma OS deve ter ao menos um serviço para gerar orçamento |
| RN06 | Valor do orçamento = Σ(preço do serviço) + Σ(quantidade × preço unitário da peça); os preços são copiados (snapshot) para a OS |
| RN07 | Estoque nunca pode ficar negativo |
| RN08 | Peças são **reservadas** ao aprovar o orçamento e **baixadas** ao iniciar a execução **[Premissa]** (simplificação: baixa direta na aprovação, ver [ADR-008](10-adrs.md)) |
| RN09 | Só é possível alterar itens da OS enquanto ela estiver em Recebida ou Em diagnóstico; itens adicionais após a aprovação exigem novo orçamento (volta a Aguardando aprovação) |
| RN10 | Transições de status seguem estritamente a [máquina de estados](03-ddd.md#36-máquina-de-estados-da-os) |
| RN11 | Orçamento rejeitado cancela a OS e libera estoque reservado |
| RN12 | Tempo de execução = `finalizada_em − execucao_iniciada_em` |
| RN13 | Cliente só acessa OS cujo CPF/CNPJ confira com o informado |
| RN14 | Serviços e peças com histórico não são excluídos fisicamente (inativação lógica) |
| RN15 | Preços e quantidades não podem ser negativos; quantidade de item ≥ 1 |

## 2.4 Critérios de aceite (fluxo principal)

```gherkin
Cenário: Abertura de OS com orçamento automático
  Dado um cliente cadastrado com CPF válido
  E um veículo vinculado a esse cliente
  Quando o atendente abre uma OS com o serviço "Troca de óleo" e 4 unidades de "Óleo 5W30"
  Então a OS é criada com status "Recebida"
  E o orçamento é calculado somando serviço e peças

Cenário: Aprovação pelo cliente
  Dado uma OS com status "Aguardando aprovação"
  Quando o cliente informa CPF e número da OS e aprova o orçamento
  Então o status muda para "Em execução"
  E o estoque das peças é debitado

Cenário: Estoque insuficiente
  Dado uma peça com 2 unidades em estoque
  Quando o cliente aprova um orçamento que exige 3 unidades
  Então a aprovação é recusada com erro de estoque insuficiente
```

## 2.5 Rastreabilidade requisito → componente

| Requisito | Contexto (DDD) | Documento |
|-----------|----------------|-----------|
| RF01–RF02, RF12–RF13, RF21 | Cadastro (Clientes/Veículos) | [03](03-ddd.md), [06](06-api.md) |
| RF03–RF11, RF16–RF17 | Ordem de Serviço | [03](03-ddd.md), [04](04-arquitetura.md) |
| RF14 | Catálogo de Serviços | [03](03-ddd.md) |
| RF15, RF18 | Estoque | [03](03-ddd.md), [05](05-modelo-de-dados.md) |
| RF19–RF20 | Identidade e Acesso | [07](07-seguranca.md) |
