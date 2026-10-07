#!/usr/bin/env bash
# Falha se a cobertura de algum domínio crítico ficar abaixo do mínimo (padrão: 80%).
# Acrescente novos pacotes críticos à lista PACOTES conforme as fases avançam.
set -euo pipefail

MINIMO="${COBERTURA_MINIMA:-80}"

PACOTES=(
  ./internal/cadastro/domain
  ./internal/shared/erros
  ./internal/shared/dinheiro
  ./internal/shared/relogio
)

saida="$(go test -cover "${PACOTES[@]}")"
echo "$saida"

falhou=0
while read -r pacote pct; do
  if awk -v p="$pct" -v m="$MINIMO" 'BEGIN { exit !(p < m) }'; then
    echo "FALHA: $pacote com cobertura ${pct}% (mínimo ${MINIMO}%)"
    falhou=1
  fi
done < <(echo "$saida" | awk '/coverage:/ { for (i = 1; i <= NF; i++) if ($i == "coverage:") { gsub("%", "", $(i+1)); print $2, $(i+1) } }')

if [ "$falhou" -ne 0 ]; then
  exit 1
fi
echo "Cobertura dos domínios críticos OK (mínimo ${MINIMO}%)."
