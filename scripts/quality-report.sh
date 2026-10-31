#!/usr/bin/env bash
# Gera os relatórios de qualidade em reports/: cobertura de testes e vulnerabilidades.
# Para incluir os testes de integração, defina TEST_DATABASE_URL (ver README).
set -euo pipefail

mkdir -p reports

go test ./... -covermode=atomic -coverprofile=reports/coverage.out
go tool cover -func=reports/coverage.out > reports/cobertura-por-funcao.txt
go tool cover -html=reports/coverage.out -o reports/cobertura.html

echo "--- cobertura dos pacotes críticos (mínimo 80%)"
bash scripts/check-coverage.sh | tee reports/cobertura-pacotes-criticos.txt

echo "--- cobertura total"
tail -1 reports/cobertura-por-funcao.txt | tee reports/cobertura-total.txt

echo "--- vulnerabilidades"
go run golang.org/x/vuln/cmd/govulncheck@latest ./... | tee reports/vulnerabilidades.txt
