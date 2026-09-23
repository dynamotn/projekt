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
#
# @see
#   - `scripts/genshdoc.awk`
#   - `scripts/lint.sh`
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=init.sh
. "${SCRIPT_DIR}/../init.sh" --modules cli

dybatpho::register_common_handlers

# @description Print the source files to document, one per line: the ones named
#   on the command line, or every documented source when none were.
#   `init.sh` ships public functions of its own, so it is documented alongside
#   the modules it loads.
# @stdout Paths of the source files
function __dybatpho_doc_sources {
  local -a _named=()
  # `dybatpho::opts::setup` collects the positional arguments into a single
  # space-joined string, so they are split back out here.
  read -r -a _named <<< "${DOC_ARGS}"
  if ((${#_named[@]})); then
    printf '%s\n' "${_named[@]}"
    return 0
  fi
  printf '%s\n' "${DYBATPHO_DIR}/init.sh" "${DYBATPHO_DIR}/src/"*.sh
}

# @description Write `doc/<module>.md` for every source.
function __dybatpho_doc_generate {
  local _src _module
  while IFS= read -r _src; do
    _module="$(basename "${_src}" .sh)"
    gawk -f "${SCRIPT_DIR}/genshdoc.awk" "${_src}" \
      > "${DYBATPHO_DIR}/doc/${_module}.md"
  done < <(__dybatpho_doc_sources)
}

# @description Compare the committed documents against freshly generated ones.
# @exitcode 0 Every document matches its source
# @exitcode 1 At least one document is stale
function __dybatpho_doc_check {
  local _generated _src _module _stale=""
  dybatpho::create_temp_dir _generated "doc-check"

  while IFS= read -r _src; do
    _module="$(basename "${_src}" .sh)"
    gawk -f "${SCRIPT_DIR}/genshdoc.awk" "${_src}" > "${_generated}/${_module}.md"
    if ! diff -q "${DYBATPHO_DIR}/doc/${_module}.md" "${_generated}/${_module}.md" > /dev/null 2>&1; then
      _stale+="doc/${_module}.md is stale relative to $(basename "${_src}")"$'\n'
    fi
  done < <(__dybatpho_doc_sources)

  if [[ -n "${_stale}" ]]; then
    dybatpho::error "Generated documentation is out of date; run scripts/doc.sh"
    printf '%s' "${_stale}" >&2
    return 1
  fi
  dybatpho::success "Generated documentation is up to date"
}

# @description Generate or check, depending on `--check`.
function __dybatpho_doc_run {
  dybatpho::require "gawk"

  if dybatpho::is true "${CHECK}"; then
    __dybatpho_doc_check
    return
  fi
  __dybatpho_doc_generate
}

# @description CLI specification for this script.
function _spec {
  dybatpho::opts::setup \
    "Generate doc/<module>.md from the shdoc comments in each source file" \
    DOC_ARGS action:"__dybatpho_doc_run"

  dybatpho::opts::arg "Source files to document; defaults to all of them" \
    SOURCE required:false variadic:true

  dybatpho::opts::flag "Fail if a committed document is stale instead of writing" \
    CHECK --check on:true off:false init:="false"

  dybatpho::opts::disp "Show help" --help action:"dybatpho::generate_help _spec"
}

dybatpho::generate_from_spec _spec "$@"
