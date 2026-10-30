#!/usr/bin/env bash
# Falha se algum pacote crítico tiver cobertura de testes abaixo do mínimo.
# A cada fase, acrescente à lista os novos pacotes com regra de negócio.
set -euo pipefail

MIN=80

CRITICAL_PACKAGES=(
  oficina-api/internal/platform/config
  oficina-api/internal/platform/httpserver
)

failed=0
for pkg in "${CRITICAL_PACKAGES[@]}"; do
  # A saída é algo como: "ok  <pacote>  0.5s  coverage: 87.5% of statements"
  line="$(go test -cover "$pkg" | grep 'coverage:')"
  pct="$(echo "$line" | sed -E 's/.*coverage: ([0-9]+(\.[0-9]+)?)%.*/\1/')"

  # awk faz a comparação de decimais, que o bash puro não faz.
  if awk -v p="$pct" -v m="$MIN" 'BEGIN { exit !(p < m) }'; then
    echo "FALHA: $pkg com ${pct}% (mínimo ${MIN}%)"
    failed=1
  else
    echo "ok:    $pkg com ${pct}%"
  fi
done

exit $failed
