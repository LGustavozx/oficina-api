# 7. Segurança

## 7.1 Autenticação e autorização

| Item | Decisão |
|------|---------|
| Mecanismo | JWT assinado (HS256) emitido em `/auth/login` |
| Claims | `sub` (id do usuário), `perfil`, `iat`, `exp`, `jti` |
| Expiração | 60 minutos **[Premissa]** (configurável) |
| Validação | Middleware verifica assinatura, `exp` e algoritmo esperado (rejeita `alg: none`) |
| Escopo | Todas as rotas `/admin/**`; rotas `/publico/**` e `/auth/login` são abertas |
| Segredo | Variável `JWT_SECRET` (mín. 32 bytes), nunca versionada |
| Autorização | Perfil `ADMIN` no MVP; middleware preparado para perfis adicionais |

**Evolução:** refresh token, revogação por `jti` e RS256 com chaves assimétricas.

## 7.2 Senhas

- Hash com **bcrypt** (custo ≥ 10).
- Mensagem de falha de login genérica ("credenciais inválidas"), sem indicar se o e-mail existe.
- Usuário administrador inicial criado por *seed* usando variáveis de ambiente.

## 7.3 Validação de dados sensíveis (RF21)

| Dado | Regra |
|------|-------|
| **CPF** | 11 dígitos, rejeita sequências repetidas, valida os 2 dígitos verificadores |
| **CNPJ** | 14 dígitos, valida os 2 dígitos verificadores |
| **Placa** | Regex `^[A-Z]{3}-?[0-9]{4}$` (antiga) ou `^[A-Z]{3}[0-9][A-Z][0-9]{2}$` (Mercosul); normalizada em maiúsculas sem hífen |
| Demais entradas | Tamanhos máximos, tipos e faixas; rejeição de campos desconhecidos no JSON |

A validação ocorre no **domínio** (objetos de valor `Documento` e `Placa`), tornando impossível criar entidades inválidas, independentemente da porta de entrada.

## 7.4 Proteção da API pública

| Ameaça | Mitigação |
|--------|-----------|
| Enumeração de OS por número sequencial | Exigir CPF/CNPJ junto do número; resposta 404 uniforme quando não conferem |
| Força bruta em documento | Rate limiting por IP (ex.: 30 req/min) |
| Vazamento de documento em logs/URL | Documento enviado em cabeçalho; logs mascaram documento (`***.456.789-**`) |
| Ação repetida (aprovar duas vezes) | Máquina de estados rejeita transição inválida |

## 7.5 Proteção contra vulnerabilidades comuns (OWASP)

| Risco (OWASP API Top 10) | Mitigação |
|--------------------------|-----------|
| Broken Object Level Authorization | Cliente só acessa OS com documento correspondente (RN13) |
| Broken Authentication | JWT validado, bcrypt, expiração |
| Excessive Data Exposure | DTOs de resposta explícitos; nunca serializar entidades/hash de senha |
| Injection | Consultas parametrizadas (`pgx`/`sqlc`); nenhuma concatenação de SQL |
| Security Misconfiguration | Imagem mínima, usuário não-root no contêiner, cabeçalhos de segurança, CORS restrito |
| Unrestricted Resource Consumption | Paginação obrigatória, limite de tamanho de body, timeouts HTTP |
| Mass Assignment | Campos permitidos definidos por DTO de entrada |

## 7.6 LGPD (Lei 13.709/2018)

| Princípio | Aplicação |
|-----------|-----------|
| Minimização | Coleta apenas nome, documento, contato e dados do veículo |
| Finalidade | Dados usados apenas para atendimento e comunicação da OS |
| Segurança | Acesso restrito por JWT, TLS em produção, logs mascarados |
| Direitos do titular | Inativação/anonimização de cliente **[Evolução]**; consulta do próprio histórico |

## 7.7 Segurança de infraestrutura

- Segredos via variáveis de ambiente (`.env` fora do controle de versão; `.env.example` versionado).
- Dockerfile *multi-stage*, imagem final mínima (distroless/alpine), execução como usuário não-root.
- Banco não exposto externamente em produção (apenas rede interna do compose).
- Comunicação HTTPS por proxy reverso em produção (fora do escopo do MVP).
- Análise estática: `gosec` e `govulncheck` no pipeline de qualidade.

## 7.9 Análise de vulnerabilidades

Exigência da entrega: o repositório deve conter o relatório ou resultado da análise.

| Ferramenta | Alvo | Comando |
|------------|------|---------|
| `govulncheck` | Dependências e stdlib com CVEs conhecidos | `govulncheck ./...` |
| `gosec` | Padrões inseguros no código Go | `gosec -fmt=json -out=gosec.json ./...` |
| `trivy` (opcional) | Imagem Docker e dependências | `trivy image oficina-api:latest` |

Os resultados são salvos em `docs/relatorios/vulnerabilidades.md`, com data, versão do commit, achados e tratamento (corrigido, aceito com justificativa ou falso positivo). A análise roda no pipeline de CI e o relatório final é gerado no commit congelado da entrega ([Entrega](11-entrega-fase1.md)).

## 7.8 Modelo simplificado de ameaças (STRIDE)

| Categoria | Cenário | Controle |
|-----------|---------|----------|
| **S**poofing | Falsificar administrador | JWT assinado + bcrypt |
| **T**ampering | Alterar preço/estoque | Autorização + transações + constraints |
| **R**epudiation | Negar ação na OS | Histórico de status com autor e origem |
| **I**nformation disclosure | Ler OS de terceiros | RN13, 404 uniforme, DTOs |
| **D**enial of service | Inundar rota pública | Rate limit, timeouts, limites de body |
| **E**levation of privilege | Cliente acessar rota admin | Prefixos separados e middleware JWT obrigatório |
