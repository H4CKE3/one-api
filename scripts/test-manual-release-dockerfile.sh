#!/usr/bin/env bash
set -euo pipefail

dockerfile="${1:-Dockerfile.manual-release}"

install_line="$(grep -n "npm .*--prefix /web/default" "${dockerfile}" | head -n 1 || true)"
build_line="$(grep -n "npm run build --prefix /web/default" "${dockerfile}" | head -n 1 || true)"

if [[ -z "${install_line}" || -z "${build_line}" ]]; then
  echo "missing expected frontend install/build lines in ${dockerfile}" >&2
  exit 1
fi

if [[ "${install_line}" == *"&"* ]]; then
  echo "frontend install step must not run in parallel: ${install_line}" >&2
  exit 1
fi

if [[ "${build_line}" == *"&"* ]]; then
  echo "frontend build step must not run in parallel: ${build_line}" >&2
  exit 1
fi

echo "manual release dockerfile frontend steps are sequential"
