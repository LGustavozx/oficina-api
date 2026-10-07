#!/usr/bin/env bash
# Falha se a cobertura de algum domínio crítico ficar abaixo do mínimo (padrão: 80%).
# Acrescente novos pacotes críticos à lista PACKAGES conforme as fases avançam.
set -euo pipefail

MIN_COVERAGE="${MIN_COVERAGE:-80}"

PACKAGES=(
  ./internal/registry/domain
  ./internal/identity/domain
  ./internal/identity/application
  ./internal/identity/infrastructure/security
  ./internal/identity/http
  ./internal/platform/httpx
  ./internal/platform/httpserver
  ./internal/shared/apperr
  ./internal/shared/money
  ./internal/shared/clock
)

output="$(go test -cover "${PACKAGES[@]}")"
echo "$output"

failed=0
while read -r pkg pct; do
  if awk -v p="$pct" -v m="$MIN_COVERAGE" 'BEGIN { exit !(p < m) }'; then
    echo "FALHA: $pkg com cobertura ${pct}% (mínimo ${MIN_COVERAGE}%)"
    failed=1
  fi
done < <(echo "$output" | awk '/coverage:/ { for (i = 1; i <= NF; i++) if ($i == "coverage:") { gsub("%", "", $(i+1)); print $2, $(i+1) } }')

if [ "$failed" -ne 0 ]; then
  exit 1
fi
echo "Cobertura dos domínios críticos OK (mínimo ${MIN_COVERAGE}%)."
