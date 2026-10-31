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

## Value objects

Valores que se definem pelo conteúdo, não por identidade. São imutáveis e só existem válidos: a validação acontece na criação.

| Value object | Pacote | Regra |
|---|---|---|
| `Document` | `registry/domain` | CPF (11 dígitos) ou CNPJ (14, numérico ou alfanumérico) com dígitos verificadores válidos |
| `Plate` | `registry/domain` | Formato antigo (`ABC1234`) ou Mercosul (`ABC1D23`) |
| `Money` | `shared/money` | Valor em centavos (`int64`), sem ponto flutuante |
