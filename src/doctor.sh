#!/usr/bin/env bash
# @file doctor.sh
# @brief Utilities for checking that the environment can run what a script loaded
# @description
#   This module answers the question a user asks after a script fails with
#   `yq isn't installed` on line 400: what else is missing? A dybatpho module
#   only calls an external tool when the caller reaches the function that needs
#   it, so a missing dependency surfaces halfway through the work instead of at
#   the start.
#
#   `dybatpho::doctor` reports the Bash version, the library version, and every
#   external command the loaded modules can call, marking each one found or
#   missing. It reads as a report rather than a failure, so a user can run it
#   before the real script and fix everything at once.
#
#   Dependencies are declared per module, split in two:
#
#   - **required** — the module's main functions cannot work without it, such as
#     `curl` for `network`;
#   - **optional** — only part of the module needs it, such as `zstd` for
#     `archive` or `gpg` for signing a release.
#
#   A dependency written as `a|b` is satisfied by any one of the alternatives:
#   `file` hashes with whichever of `sha256sum`, `shasum`, or `openssl` exists.
#
#   Only missing **required** dependencies make `dybatpho::doctor` fail, so an
#   optional entry is information rather than a problem.
# @see
#   - `example/doctor_ops.sh`
#   - `scripts/bundle.sh`
: "${DYBATPHO_DIR:?DYBATPHO_DIR must be set. Please source dybatpho/init.sh before other scripts from dybatpho.}"

# @env DYBATPHO_BASH_MINIMUM string Lowest Bash version the library supports
DYBATPHO_BASH_MINIMUM="4.3"

# External commands each module can call. A module absent from both maps calls
# nothing outside Bash and the POSIX tools every system ships. Keep an entry
# here on the module that runs the command itself: a module pulls its own
# dependencies in through the registry, and `dybatpho::doctor` reports those
# modules on their own rows.
# @env DYBATPHO_DOCTOR_REQUIRED array Required external commands per module
declare -gA DYBATPHO_DOCTOR_REQUIRED=(
  [ai]="curl"
  [archive]="tar"
  [git]="git"
  [json]="yq"
  [network]="curl"
)
# @env DYBATPHO_DOCTOR_OPTIONAL array Optional external commands per module
declare -gA DYBATPHO_DOCTOR_OPTIONAL=(
  [agent]="git"
  [ai]="claude|llm|ollama"
  [archive]="unzip zip gzip bzip2 xz zstd"
  [config]="jq yq"
  [file]="sha256sum|shasum|openssl"
  [json]="jq"
  [logging]="python3"
  [network]="sha256sum md5sum"
  [os]="hostname nproc|sysctl|getconf tput"
  [release]="gpg"
)

#######################################
# @description Return success when a dependency spec is satisfied.
#   A spec is one command name, or several separated by `|` when any one of
#   them will do.
# @arg $1 string Dependency spec, such as `curl` or `sha256sum|shasum`
# @arg $2 string Optional name of the variable that receives the resolved path
# @set The named variable, to the path of the command that satisfied the spec
# @exitcode 0 At least one of the alternatives is installed
# @exitcode 1 None of the alternatives is installed
#######################################
function __dybatpho_doctor_resolve {
  local spec="$1"
  # The locals are named apart from anything a caller is likely to pass as the
  # output variable, which `printf -v` would otherwise write to the local copy.
  local __doctor_command __doctor_path
  for __doctor_command in ${spec//|/ }; do
    __doctor_path="$(command -v "${__doctor_command}" 2> /dev/null || true)"
    if [[ -n "${__doctor_path}" ]]; then
      [[ -z "${2-}" ]] || printf -v "$2" '%s' "${__doctor_path}"
      return 0
    fi
  done
  [[ -z "${2-}" ]] || printf -v "$2" '%s' ""
  return 1
}

#######################################
# @description Print the external commands a module can call.
# @example
#   dybatpho::doctor_requirements archive required
#
# @arg $1 string Module name
# @arg $2 string Kind, one of `all` (default), `required`, or `optional`
# @stdout One dependency spec per line, required specs first
# @exitcode 0 Print the dependencies, including nothing for a module that has none
# @exitcode 1 Stop the script when the module is unknown or the kind is invalid
#######################################
function dybatpho::doctor_requirements {
  local module
  dybatpho::expect_args module -- "$@"
  local kind="${2:-all}"
  __dybatpho_module_exists "${module}" \
    || dybatpho::die "${FUNCNAME[0]}: Unknown module '${module}'"
  local specs=""
  case "${kind}" in
    all) specs="${DYBATPHO_DOCTOR_REQUIRED[${module}]-} ${DYBATPHO_DOCTOR_OPTIONAL[${module}]-}" ;;
    required) specs="${DYBATPHO_DOCTOR_REQUIRED[${module}]-}" ;;
    optional) specs="${DYBATPHO_DOCTOR_OPTIONAL[${module}]-}" ;;
    *) dybatpho::die "${FUNCNAME[0]}: Unknown kind '${kind}', expected all, required or optional" ;;
  esac
  local spec
  for spec in ${specs}; do
    printf '%s\n' "${spec}"
  done
}

#######################################
# @description Return success when the running Bash is new enough for the library.
# @exitcode 0 Bash is at least `DYBATPHO_BASH_MINIMUM`
# @exitcode 1 Bash is older than the supported minimum
#######################################
function dybatpho::doctor_bash_supported {
  ((BASH_VERSINFO[0] > 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] >= 3)))
}

#######################################
# @description Resolve the module list a report covers.
# @arg $1 string Name of the array variable that receives the module names
# @arg $2 string Scope, either `loaded`, `all`, or an explicit module list
# @set The named array, to module names in registry or load order
# @exitcode 1 Stop the script when an explicitly named module is unknown
#######################################
function __dybatpho_doctor_scope {
  local -n __scope_out="$1"
  local scope="$2"
  local names
  case "${scope}" in
    loaded) names="${DYBATPHO_LOADED_MODULES}" ;;
    all) names="${DYBATPHO_CORE_MODULES} ${DYBATPHO_OPTIONAL_MODULES}" ;;
    *) names="${scope//,/ }" ;;
  esac
  __scope_out=()
  local module
  for module in ${names}; do
    __dybatpho_module_exists "${module}" \
      || dybatpho::die "dybatpho::doctor: Unknown module '${module}'"
    __scope_out+=("${module}")
  done
}

#######################################
# @description Escape a value for use inside a JSON string.
#   The report is written without `jq`, because a diagnostic that needs a tool
#   the user may be missing is of no use.
# @arg $1 string Raw value
# @stdout The value with the characters JSON reserves escaped
#######################################
function __dybatpho_doctor_json_escape {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\t'/\\t}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  printf '%s' "${value}"
}

#######################################
# @description Collect every dependency row a scope produces.
#   A row is `module<TAB>spec<TAB>kind<TAB>status<TAB>path`, which keeps the
#   text and JSON renderers reading the same data.
# @arg $1 string Name of the array variable that receives the rows
# @arg $@ string Module names to inspect
# @set The named array, to one row per dependency
#######################################
function __dybatpho_doctor_rows {
  local -n __rows_out="$1"
  shift
  __rows_out=()
  local module kind specs spec status path
  for module in "$@"; do
    for kind in required optional; do
      case "${kind}" in
        required) specs="${DYBATPHO_DOCTOR_REQUIRED[${module}]-}" ;;
        optional) specs="${DYBATPHO_DOCTOR_OPTIONAL[${module}]-}" ;;
      esac
      for spec in ${specs}; do
        if __dybatpho_doctor_resolve "${spec}" path; then
          status="ok"
        else
          status="missing"
        fi
        __rows_out+=("${module}"$'\t'"${spec}"$'\t'"${kind}"$'\t'"${status}"$'\t'"${path}")
      done
    done
  done
}

#######################################
# @description Print the report as aligned text.
# @arg $1 string Name of the array variable holding the rows
# @arg $@ string Module names covered by the report
# @stdout The environment summary, the dependency table, and a closing summary
#######################################
function __dybatpho_doctor_report_text {
  local -n __rows_in="$1"
  shift
  local bash_status="ok"
  dybatpho::doctor_bash_supported || bash_status="unsupported"
  printf 'dybatpho %s (%s)\n' "$(dybatpho::version)" "${DYBATPHO_DIR}"
  printf 'bash     %s [%s, minimum %s]\n' \
    "${BASH_VERSION}" "${bash_status}" "${DYBATPHO_BASH_MINIMUM}"
  printf 'platform %s/%s\n' "$(uname -s)" "$(uname -m)"
  printf 'modules  %s\n' "$*"

  if ((${#__rows_in[@]} == 0)); then
    printf '\nNo external dependency is needed by these modules.\n'
    return 0
  fi

  # Width of the two variable columns, so the table stays readable whatever the
  # module and command names are.
  local module_width=6 spec_width=10 row module spec
  for row in "${__rows_in[@]}"; do
    IFS=$'\t' read -r module spec _ _ _ <<< "${row}"
    ((${#module} <= module_width)) || module_width=${#module}
    ((${#spec} <= spec_width)) || spec_width=${#spec}
  done

  local kind status path
  printf '\n%-*s  %-*s  %-8s  %s\n' \
    "${module_width}" "MODULE" "${spec_width}" "DEPENDENCY" "KIND" "STATUS"
  for row in "${__rows_in[@]}"; do
    IFS=$'\t' read -r module spec kind status path <<< "${row}"
    printf '%-*s  %-*s  %-8s  %s\n' \
      "${module_width}" "${module}" "${spec_width}" "${spec}" "${kind}" \
      "${status}${path:+ (${path})}"
  done
}

#######################################
# @description Print the report as a single JSON object.
# @arg $1 string Name of the array variable holding the rows
# @arg $@ string Module names covered by the report
# @stdout One JSON object describing the environment and every dependency
#######################################
function __dybatpho_doctor_report_json {
  local -n __rows_in="$1"
  shift
  local bash_ok="false"
  dybatpho::doctor_bash_supported && bash_ok="true"
  printf '{"version":"%s"' "$(__dybatpho_doctor_json_escape "$(dybatpho::version)")"
  printf ',"directory":"%s"' "$(__dybatpho_doctor_json_escape "${DYBATPHO_DIR}")"
  printf ',"bash":{"version":"%s","minimum":"%s","ok":%s}' \
    "$(__dybatpho_doctor_json_escape "${BASH_VERSION}")" \
    "${DYBATPHO_BASH_MINIMUM}" "${bash_ok}"
  printf ',"platform":{"system":"%s","machine":"%s"}' \
    "$(__dybatpho_doctor_json_escape "$(uname -s)")" \
    "$(__dybatpho_doctor_json_escape "$(uname -m)")"
  local module first=1
  printf ',"modules":['
  for module in "$@"; do
    ((first)) || printf ','
    first=0
    printf '"%s"' "$(__dybatpho_doctor_json_escape "${module}")"
  done
  printf ']'
  local row spec kind status path
  first=1
  printf ',"dependencies":['
  for row in "${__rows_in[@]}"; do
    IFS=$'\t' read -r module spec kind status path <<< "${row}"
    ((first)) || printf ','
    first=0
    printf '{"module":"%s","dependency":"%s","kind":"%s","status":"%s","path":"%s"}' \
      "$(__dybatpho_doctor_json_escape "${module}")" \
      "$(__dybatpho_doctor_json_escape "${spec}")" \
      "${kind}" "${status}" \
      "$(__dybatpho_doctor_json_escape "${path}")"
  done
  printf ']'
}

#######################################
# @description Report the environment the loaded modules need, and what is missing.
# @example
#   dybatpho::doctor              # the modules this shell loaded
#   dybatpho::doctor --all        # every module in the registry
#   dybatpho::doctor --modules "json git" --json
#
# @arg $@ string Options: `--all`, `--modules <list>`, `--json`, `--quiet`
# @stdout The report, as aligned text or as one JSON object with `--json`
# @exitcode 0 Every required dependency is installed and Bash is supported
# @exitcode 1 A required dependency is missing, Bash is too old, or an option is invalid
#######################################
function dybatpho::doctor {
  local scope="loaded" format="text" quiet="false"
  while (($#)); do
    case "$1" in
      --all) scope="all" ;;
      --modules)
        [[ -n "${2-}" ]] || dybatpho::die "${FUNCNAME[0]}: --modules expects a module list"
        scope="$2"
        shift
        ;;
      --json) format="json" ;;
      --quiet) quiet="true" ;;
      *) dybatpho::die "${FUNCNAME[0]}: Unknown option '$1'" ;;
    esac
    shift
  done

  local modules=()
  __dybatpho_doctor_scope modules "${scope}"
  local rows=()
  __dybatpho_doctor_rows rows ${modules[@]+"${modules[@]}"}

  # Collect what is missing before printing, so the text summary and the exit
  # code describe the same run.
  local row spec kind status missing_required=() missing_optional=()
  for row in "${rows[@]}"; do
    IFS=$'\t' read -r _ spec kind status _ <<< "${row}"
    [[ "${status}" == "missing" ]] || continue
    if [[ "${kind}" == "required" ]]; then
      missing_required+=("${spec}")
    else
      missing_optional+=("${spec}")
    fi
  done

  local healthy=0
  ((${#missing_required[@]} == 0)) || healthy=1
  dybatpho::doctor_bash_supported || healthy=1

  if [[ "${quiet}" == "true" ]]; then
    return "${healthy}"
  fi

  if [[ "${format}" == "json" ]]; then
    local ok="true"
    ((healthy == 0)) || ok="false"
    __dybatpho_doctor_report_json rows ${modules[@]+"${modules[@]}"}
    printf ',"ok":%s}\n' "${ok}"
    return "${healthy}"
  fi

  __dybatpho_doctor_report_text rows ${modules[@]+"${modules[@]}"}
  printf '\n'
  if ((${#missing_optional[@]} > 0)); then
    printf 'Optional, some functions are unavailable: %s\n' "${missing_optional[*]}"
  fi
  if ((${#missing_required[@]} > 0)); then
    printf 'Missing required: %s\n' "${missing_required[*]}"
  fi
  dybatpho::doctor_bash_supported \
    || printf 'Bash %s is older than the supported minimum %s\n' \
      "${BASH_VERSION}" "${DYBATPHO_BASH_MINIMUM}"
  ((healthy)) || printf 'No required dependency is missing.\n'
  return "${healthy}"
}
