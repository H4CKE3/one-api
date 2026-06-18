#!/usr/bin/env bash
set -euo pipefail

dockerfile="${1:-Dockerfile.manual-release}"

for line in \
  "$(grep -n "npm .*--prefix /web/default" "${dockerfile}" | head -n 1 || true)" \
  "$(grep -n "npm .*--prefix /web/berry" "${dockerfile}" | head -n 1 || true)" \
  "$(grep -n "npm .*--prefix /web/air" "${dockerfile}" | head -n 1 || true)" \
  "$(grep -n "npm run build --prefix /web/default" "${dockerfile}" | head -n 1 || true)" \
  "$(grep -n "npm run build --prefix /web/berry" "${dockerfile}" | head -n 1 || true)" \
  "$(grep -n "npm run build --prefix /web/air" "${dockerfile}" | head -n 1 || true)"
do
  if [[ -z "${line}" ]]; then
    echo "missing expected frontend install/build lines in ${dockerfile}" >&2
    exit 1
  fi

  if [[ "${line}" == *"&"* ]]; then
    echo "frontend step must not run in parallel: ${line}" >&2
    exit 1
  fi
done

echo "manual release dockerfile frontend steps are sequential"
