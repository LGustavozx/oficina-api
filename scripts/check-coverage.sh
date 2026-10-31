#!/usr/bin/env bash
# Falha se algum pacote crítico tiver cobertura abaixo do mínimo.
# Acrescente à lista os novos pacotes com regra de negócio a cada fase.
set -euo pipefail

MIN=80

CRITICAL_PACKAGES=(
  oficina-api/internal/platform/config
  oficina-api/internal/platform/httpserver
  oficina-api/internal/shared/apperr
  oficina-api/internal/shared/money
  oficina-api/internal/shared/clock
  oficina-api/internal/registry/domain
  oficina-api/internal/identity/domain
  oficina-api/internal/identity/application
  oficina-api/internal/identity/infrastructure
  oficina-api/internal/identity/http
)

failed=0
for pkg in "${CRITICAL_PACKAGES[@]}"; do
  line="$(go test -cover "$pkg" | grep 'coverage:')"
  pct="$(echo "$line" | sed -E 's/.*coverage: ([0-9]+(\.[0-9]+)?)%.*/\1/')"

  if awk -v p="$pct" -v m="$MIN" 'BEGIN { exit !(p < m) }'; then
    echo "FALHA: $pkg com ${pct}% (mínimo ${MIN}%)"
    failed=1
  else
    echo "ok:    $pkg com ${pct}%"
  fi
done

exit $failed
