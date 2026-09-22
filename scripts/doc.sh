#!/usr/bin/env bash
# @file doc.sh
# @brief Generate documentation of dybatpho
# @description
#   Writes `doc/<module>.md` from the shdoc comments in each source file.
#
#   `--check` generates into a temporary directory and compares instead of
#   writing, which is what CI needs: the committed documentation is generated,
#   so it can silently fall behind the source it describes. Locally the
#   `gen-doc` pre-commit hook rewrites the files and a dirty tree makes the
#   drift obvious; a pull request has no such signal without this flag.
#
# @example
#   scripts/doc.sh                 # regenerate every document
#   scripts/doc.sh --check         # fail if a committed document is stale
#   scripts/doc.sh src/logging.sh  # regenerate one module
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=init.sh
. "${SCRIPT_DIR}/../init.sh"
dybatpho::require "gawk"

check=false
if [[ "${1-}" == "--check" ]]; then
  check=true
  shift
fi

if (($#)); then
  sources=("$@")
else
  # `init.sh` ships public functions of its own, so it is documented alongside
  # the modules it loads.
  sources=("${DYBATPHO_DIR}/init.sh" "${DYBATPHO_DIR}/src/"*.sh)
fi

if dybatpho::is true "${check}"; then
  stale=""
  generated=""
  dybatpho::create_temp_dir generated "doc-check"
  for src in "${sources[@]}"; do
    module="$(basename "${src}" .sh)"
    gawk -f "${SCRIPT_DIR}/genshdoc.awk" "${src}" > "${generated}/${module}.md"
    if ! diff -q "${DYBATPHO_DIR}/doc/${module}.md" "${generated}/${module}.md" > /dev/null 2>&1; then
      stale+="doc/${module}.md is stale relative to $(basename "${src}")"$'\n'
    fi
  done

  if [[ -n "${stale}" ]]; then
    dybatpho::error "Generated documentation is out of date; run scripts/doc.sh"
    printf '%s' "${stale}" >&2
    exit 1
  fi
  dybatpho::success "Generated documentation is up to date"
  exit 0
fi

for src in "${sources[@]}"; do
  module="$(basename "${src}" .sh)"
  gawk \
    -f "${SCRIPT_DIR}/genshdoc.awk" \
    "${src}" > "${DYBATPHO_DIR}/doc/${module}.md"
done
