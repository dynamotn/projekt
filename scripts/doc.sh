#!/usr/bin/env bash
# @file doc.sh
# @brief Generate documentation of dybatpho
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=init.sh
. "${SCRIPT_DIR}/../init.sh"
dybatpho::require "gawk"

if (($#)); then
  sources=("$@")
else
  # `init.sh` ships public functions of its own, so it is documented alongside
  # the modules it loads.
  sources=("${DYBATPHO_DIR}/init.sh" "${DYBATPHO_DIR}/src/"*.sh)
fi

for src in "${sources[@]}"; do
  module="$(basename "${src}" .sh)"
  gawk \
    -f "${SCRIPT_DIR}/genshdoc.awk" \
    "${src}" > "${DYBATPHO_DIR}/doc/${module}.md"
done
