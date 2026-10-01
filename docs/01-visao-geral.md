# 1. Visão geral

## 1.1 Contexto

Uma oficina mecânica de médio porte, especializada em manutenção de veículos, enfrenta dificuldades para expandir seus serviços com qualidade e eficiência. O atendimento, o diagnóstico, a execução e a entrega dos veículos são feitos com anotações manuais e planilhas.

## 1.2 Problemas identificados

| # | Problema | Consequência |
|---|----------|--------------|
| P1 | Erros na priorização dos atendimentos | Atrasos e insatisfação do cliente |
| P2 | Falhas no controle de peças e insumos | Falta de material, compras urgentes, perdas |
| P3 | Dificuldade em acompanhar o status dos serviços | Retrabalho de comunicação com o cliente |
| P4 | Perda de histórico de clientes e veículos | Diagnósticos sem contexto, baixa fidelização |
| P5 | Ineficiência no fluxo de orçamentos e autorizações | Execução parada aguardando aprovação |

## 1.3 Objetivo

Desenvolver a primeira versão (MVP) do back-end do **Sistema Integrado de Atendimento e Execução de Serviços**, permitindo:

- gestão de ordens de serviço, clientes, veículos, serviços e peças;
- acompanhamento do andamento da OS pelo cliente via API;
- aprovação de orçamento pelo cliente;
- gestão interna eficiente e segura.

### Rastreabilidade problema → solução

| Problema | Resposta do sistema |
|----------|---------------------|
| P1 | Listagem de OS por status e data de entrada; fila ordenável |
| P2 | CRUD de peças com controle de estoque e reserva/baixa automática |
| P3 | Máquina de estados da OS e consulta pública de andamento |
| P4 | Cadastro persistente de clientes e veículos com histórico de OS |
| P5 | Orçamento automático e fluxo de aprovação/rejeição via API |

## 1.4 Escopo do MVP

**Dentro do escopo**

- API REST do back-end (monolito em Go).
- Fluxos de criação e acompanhamento da OS.
- CRUD administrativo de clientes, veículos, serviços e peças/insumos.
- Autenticação JWT para rotas administrativas.
- Validação de CPF/CNPJ e placa.
- Métrica de tempo médio de execução.
- Testes automatizados, Docker e documentação Swagger.

**Fora do escopo (evoluções futuras)**

- Aplicativo móvel / front-end (apenas a API é entregue).
- Envio real de e-mail/SMS/WhatsApp (será simulado por uma porta de notificação).
- Emissão fiscal, pagamentos e financeiro.
- Gestão de fornecedores e compras.
- Agendamento de atendimentos.
- Decomposição em microsserviços.

## 1.5 Atores

| Ator | Descrição | Acesso |
|------|-----------|--------|
| **Administrador / Atendente** | Funcionário que cadastra, abre e gerencia OS | API administrativa (JWT) |
| **Mecânico** | Executa diagnóstico e serviços **[Premissa]**: atua pelo mesmo perfil administrativo no MVP | API administrativa (JWT) |
| **Cliente** | Proprietário do veículo; acompanha e aprova orçamento | API pública (CPF/CNPJ + nº da OS) |

## 1.6 Premissas

- **[Premissa 1]** Uma única oficina (sem multi-tenant).
- **[Premissa 2]** Autenticação JWT restrita ao perfil administrativo. O cliente não tem login: identifica-se com CPF/CNPJ + número da OS.
- **[Premissa 3]** O envio do orçamento ao cliente é simulado (log/registro de notificação); a aprovação ocorre por endpoint público.
- **[Premissa 4]** Adiciona-se o status **Cancelada** aos seis definidos no enunciado, para tratar orçamento rejeitado.
- **[Premissa 5]** Valores monetários em BRL, armazenados em centavos (inteiro).
- **[Premissa 6]** Um mesmo veículo pertence a um único cliente por vez; a troca de proprietário é feita por atualização.

## 1.7 Glossário

Ver [linguagem ubíqua](03-ddd.md#31-linguagem-ubíqua).
