# 6. API REST

- **Base URL:** `/api/v1`
- **Formato:** JSON (UTF-8)
- **Documentação interativa:** Swagger UI em `/swagger/index.html` (OpenAPI gerado por anotações `swaggo`)
- **Datas:** ISO 8601 UTC. **Valores monetários:** inteiros em centavos.

## 6.1 Grupos de rotas

| Grupo | Prefixo | Autenticação |
|-------|---------|--------------|
| Autenticação | `/auth` | Pública |
| Administrativa | `/admin` | **JWT (Bearer)** |
| Pública do cliente | `/publico` | CPF/CNPJ + nº da OS |
| Operacional | `/health`, `/ready` | Pública |

## 6.2 Autenticação

| Método | Rota | Descrição |
|--------|------|-----------|
| POST | `/auth/login` | Recebe `{email, senha}` e devolve `{access_token, expires_in}` |

Uso: `Authorization: Bearer <token>`.

## 6.3 Rotas administrativas (JWT)

### Clientes (RF12)

| Método | Rota | Descrição |
|--------|------|-----------|
| POST | `/admin/clientes` | Cria cliente (valida CPF/CNPJ) |
| GET | `/admin/clientes` | Lista com paginação e filtro por nome/documento |
| GET | `/admin/clientes/{id}` | Detalha |
| PUT | `/admin/clientes/{id}` | Atualiza |
| DELETE | `/admin/clientes/{id}` | Inativa |
| GET | `/admin/clientes/{id}/veiculos` | Veículos do cliente |
| GET | `/admin/clientes/{id}/ordens-servico` | Histórico de OS |

### Veículos (RF13)

| Método | Rota | Descrição |
|--------|------|-----------|
| POST | `/admin/veiculos` | Cria (valida placa, ano) |
| GET | `/admin/veiculos` | Lista |
| GET | `/admin/veiculos/{id}` | Detalha |
| PUT | `/admin/veiculos/{id}` | Atualiza |
| DELETE | `/admin/veiculos/{id}` | Inativa |

### Serviços (RF14)

| Método | Rota | Descrição |
|--------|------|-----------|
| POST / GET / GET{id} / PUT{id} / DELETE{id} | `/admin/servicos` | CRUD do catálogo |

### Peças e insumos (RF15, RF18)

| Método | Rota | Descrição |
|--------|------|-----------|
| POST / GET / GET{id} / PUT{id} / DELETE{id} | `/admin/pecas` | CRUD |
| POST | `/admin/pecas/{id}/entradas` | Registra entrada de estoque |
| GET | `/admin/pecas/{id}/movimentacoes` | Extrato de movimentações |
| GET | `/admin/pecas?abaixo_do_minimo=true` | Peças com estoque baixo |

### Ordens de serviço (RF01–RF09, RF16)

| Método | Rota | Descrição | Transição |
|--------|------|-----------|-----------|
| POST | `/admin/ordens-servico` | Abre OS (`documento_cliente` ou `cliente_id`, `veiculo_id`, `descricao_problema`) | → Recebida |
| GET | `/admin/ordens-servico` | Lista (filtros: `status`, `cliente_id`, `de`, `ate`, paginação, ordenação) | — |
| GET | `/admin/ordens-servico/{id}` | Detalha com itens, total e histórico | — |
| POST | `/admin/ordens-servico/{id}/diagnostico` | Inicia diagnóstico | Recebida → Em diagnóstico |
| POST | `/admin/ordens-servico/{id}/servicos` | Inclui serviço | — |
| DELETE | `/admin/ordens-servico/{id}/servicos/{itemId}` | Remove serviço | — |
| POST | `/admin/ordens-servico/{id}/pecas` | Inclui peça `{peca_id, quantidade}` | — |
| DELETE | `/admin/ordens-servico/{id}/pecas/{itemId}` | Remove peça | — |
| GET | `/admin/ordens-servico/{id}/orcamento` | Calcula/consulta orçamento | — |
| POST | `/admin/ordens-servico/{id}/orcamento` | Envia orçamento ao cliente | Em diagnóstico → Aguardando aprovação |
| POST | `/admin/ordens-servico/{id}/finalizar` | Finaliza | Em execução → Finalizada |
| POST | `/admin/ordens-servico/{id}/entregar` | Entrega o veículo | Finalizada → Entregue |
| POST | `/admin/ordens-servico/{id}/cancelar` | Cancela | Recebida/Diagnóstico → Cancelada |

### Métricas (RF17)

| Método | Rota | Descrição |
|--------|------|-----------|
| GET | `/admin/metricas/tempo-medio-execucao` | Parâmetros opcionais: `de`, `ate`, `servico_id`. Retorna `{tempo_medio_segundos, amostra}` |

## 6.4 Rotas públicas do cliente (RF07, RF10)

O cliente informa o CPF/CNPJ no cabeçalho `X-Documento` (evita expor o dado em URL/logs). Combinação inválida retorna **404** para não revelar a existência da OS.

| Método | Rota | Descrição |
|--------|------|-----------|
| GET | `/publico/ordens-servico/{numero}` | Status atual, histórico resumido e orçamento |
| POST | `/publico/ordens-servico/{numero}/aprovacao` | Aprova orçamento (Aguardando aprovação → Em execução) |
| POST | `/publico/ordens-servico/{numero}/rejeicao` | Rejeita orçamento (→ Cancelada) |

Mitigação de abuso: rate limiting por IP e resposta uniforme em falha ([Segurança](07-seguranca.md)).

## 6.5 Exemplos

**Login**

```http
POST /api/v1/auth/login
{ "email": "admin@oficina.com", "senha": "********" }
→ 200 { "access_token": "eyJ...", "token_type": "Bearer", "expires_in": 3600 }
```

**Abrir OS**

```http
POST /api/v1/admin/ordens-servico
Authorization: Bearer eyJ...
{ "documento_cliente": "12345678909", "veiculo_id": "6f1c...", "descricao_problema": "Barulho na suspensão" }
→ 201 { "id": "b2a7...", "numero": 1042, "status": "RECEBIDA" }
```

**Consulta pelo cliente**

```http
GET /api/v1/publico/ordens-servico/1042
X-Documento: 12345678909
→ 200 {
  "numero": 1042,
  "status": "AGUARDANDO_APROVACAO",
  "veiculo": { "placa": "ABC1D23", "modelo": "Onix" },
  "orcamento": { "total_centavos": 48500, "itens": [ ... ] },
  "historico": [ { "status": "RECEBIDA", "em": "2026-09-29T12:00:00Z" } ]
}
```

## 6.6 Convenções

- **Paginação:** `?pagina=1&tamanho=20`; resposta com `{ "dados": [], "pagina": 1, "tamanho": 20, "total": 134 }`.
- **Versionamento:** prefixo `/v1`.
- **Erros:** formato de [4.6](04-arquitetura.md#46-tratamento-de-erros).
- **Idempotência:** aprovação/rejeição repetida retorna 409 com código `TRANSICAO_INVALIDA`.
- **Rastreabilidade:** cabeçalho `X-Request-ID` devolvido em toda resposta.
