# DDD

## Linguagem ubíqua

O vocabulário do negócio é em português na documentação; no código, em inglês.

| Termo (negócio) | Código |
|---|---|
| Cliente | `Customer` |
| Veículo | `Vehicle` |
| Placa | `Plate` |
| CPF/CNPJ | `Document` |
| Ordem de Serviço (OS) | `WorkOrder` |
| Serviço | `Service` |
| Peça / insumo | `Part` |
| Orçamento | `Quote` |
| Estoque | `Stock` |
| Movimentação de estoque | `StockMovement` |
| Status da OS | `Status` |

## Ordem de serviço

```
RECEIVED ──> IN_DIAGNOSIS ──> AWAITING_APPROVAL ──> IN_EXECUTION ──> COMPLETED ──> DELIVERED
                                      │
                                      └──> CANCELLED   (orçamento recusado)
```

| Status | Termo do negócio |
|---|---|
| `RECEIVED` | Recebida |
| `IN_DIAGNOSIS` | Em diagnóstico |
| `AWAITING_APPROVAL` | Aguardando aprovação |
| `IN_EXECUTION` | Em execução |
| `COMPLETED` | Finalizada |
| `DELIVERED` | Entregue |
| `CANCELLED` | Cancelada (acrescentado: sem ele, um orçamento recusado deixaria a OS presa) |

Regras:

- O status só muda pelas ações do domínio (`StartDiagnosis`, `SendQuote`, `Approve`, `Reject`, `Complete`, `Deliver`), e cada uma só vale a partir de um status específico.
- Os itens (serviços e peças) só podem ser trocados antes do envio do orçamento. Cada item guarda o nome e o preço do momento em que entrou.
- O orçamento é sempre calculado: soma de preço × quantidade dos itens.
- A aprovação inicia a execução e dá baixa no estoque, numa única transação. Os marcos `started_at`, `completed_at` e `delivered_at` permitem medir o tempo de execução.

## Value objects

Valores que se definem pelo conteúdo, não por identidade. São imutáveis e só existem válidos: a validação acontece na criação.

| Value object | Pacote | Regra |
|---|---|---|
| `Document` | `registry/domain` | CPF (11 dígitos) ou CNPJ (14, numérico ou alfanumérico) com dígitos verificadores válidos |
| `Plate` | `registry/domain` | Formato antigo (`ABC1234`) ou Mercosul (`ABC1D23`) |
| `Money` | `shared/money` | Valor em centavos (`int64`), sem ponto flutuante |
