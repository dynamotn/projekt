#!/usr/bin/env bash
# @file semver_ops.sh
# @brief Example showing semver utilities
# @description Demonstrates dybatpho::semver_valid, semver_parse, semver_compare, semver_release_type, semver_bump
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=init.sh
. "${SCRIPTDIR}/../init.sh" --modules semver

dybatpho::register_common_handlers

function _demo_valid {
  dybatpho::header "SEMVER VALID"
  local versions=("1.2.3" "v2.0.0-rc.1" "1.0.0+build.42" "1.2" "not-a-version" "")
  local v
  for v in "${versions[@]}"; do
    dybatpho::info "'${v}': $(dybatpho::semver_valid "${v}" && echo valid || echo invalid)"
  done
}

function _demo_parse {
  dybatpho::header "SEMVER PARSE"
  local version="1.4.2-beta.3+exp.sha.abc123"
  dybatpho::info "Input: ${version}"
  local -a parts
  mapfile -t parts < <(dybatpho::semver_parse "${version}")
  dybatpho::print "  major         : ${parts[0]}"
  dybatpho::print "  minor         : ${parts[1]}"
  dybatpho::print "  patch         : ${parts[2]}"
  dybatpho::print "  pre-release   : ${parts[3]:-<none>}"
  dybatpho::print "  build-metadata: ${parts[4]:-<none>}"
}

function _demo_compare {
  dybatpho::header "SEMVER COMPARE"
  local pairs=(
    "1.0.0  1.0.0"
    "2.0.0  1.9.9"
    "1.0.0  2.0.0"
    "1.0.0-alpha  1.0.0"
    "1.0.0-alpha  1.0.0-alpha.1"
    "1.0.0-beta.2  1.0.0-beta.11"
    "1.0.0+build.1  1.0.0+build.2"
  )
  local pair v1 v2 result
  for pair in "${pairs[@]}"; do
    read -r v1 v2 <<<"${pair}"
    result=$(dybatpho::semver_compare "${v1}" "${v2}")
    case "${result}" in
    -1) dybatpho::info "${v1}  <  ${v2}" ;;
    0) dybatpho::info "${v1}  =  ${v2}" ;;
    1) dybatpho::info "${v1}  >  ${v2}" ;;
    esac
  done
}

function _demo_release_type {
  dybatpho::header "SEMVER RELEASE TYPE"
  local pairs=(
    "1.0.0  2.0.0"
    "1.0.0  1.1.0"
    "1.0.0  1.0.1"
    "1.0.0  1.0.0-rc.1"
    "1.0.0-rc.1  1.0.0-rc.2"
    "1.0.0+build.1  1.0.0+build.2"
    "1.0.0  1.0.0"
  )
  local pair old new rtype
  for pair in "${pairs[@]}"; do
    read -r old new <<<"${pair}"
    rtype=$(dybatpho::semver_release_type "${old}" "${new}")
    dybatpho::info "${old}  ->  ${new}  :  ${rtype}"
  done
}

function _demo_bump {
  dybatpho::header "SEMVER BUMP"
  local base="1.4.2-alpha.1+build.5"
  dybatpho::info "Base version: ${base}"
  dybatpho::print "  bump major             : $(dybatpho::semver_bump "${base}" major)"
  dybatpho::print "  bump minor             : $(dybatpho::semver_bump "${base}" minor)"
  dybatpho::print "  bump patch             : $(dybatpho::semver_bump "${base}" patch)"
  dybatpho::print "  bump major + pre rc.1  : $(dybatpho::semver_bump "${base}" major "rc.1")"
  dybatpho::print "  bump patch + build meta: $(dybatpho::semver_bump "${base}" patch "" "sha.abc123")"
  dybatpho::print "  bump minor + both      : $(dybatpho::semver_bump "${base}" minor "beta.1" "exp.42")"
}

function _demo_ranges {
  dybatpho::header "RANGE CONSTRAINTS"
  # The question a dependency check actually asks, written as the requirement
  # itself rather than as a chain of comparisons.
  local pair version range
  for pair in "1.4.2|^1.2" "2.0.0|^1.2" "1.2.9|~1.2.3" "1.5.0|>=1.2 <1.9" "18.1.0|>=18" "3.1.0|^1.0 || ^3.0"; do
    version="${pair%%|*}"
    range="${pair#*|}"
    if dybatpho::semver_satisfies "${version}" "${range}"; then
      dybatpho::print "  $(printf '%-8s' "${version}") satisfies   ${range}"
    else
      dybatpho::print "  $(printf '%-8s' "${version}") misses      ${range}"
    fi
  done

  dybatpho::info "A pre-release stays out of a range that never named one:"
  if dybatpho::semver_satisfies "2.0.0-alpha" "^1.0.0"; then
    dybatpho::warn "  2.0.0-alpha satisfied ^1.0.0, which would be a nasty surprise"
  else
    dybatpho::print "  2.0.0-alpha does not satisfy ^1.0.0"
  fi

  # The shape a real check takes.
  local required=">=1.2"
  local installed="1.4.2"
  dybatpho::semver_satisfies "${installed}" "${required}" \
    || dybatpho::die "Need ${required}, found ${installed}"
  dybatpho::success "Dependency check passed: ${installed} satisfies ${required}"
}

function _demo_ordering {
  dybatpho::header "ORDERING VERSIONS"
  # String order would put 1.10.0 before 1.9.0; version order does not.
  dybatpho::info "Sorted: $(dybatpho::semver_sort 1.10.0 1.9.0 2.0.0 1.2.3 | tr '\n' ' ')"
  dybatpho::info "With pre-releases: $(dybatpho::semver_sort 2.0.0 2.0.0-rc.1 2.0.0-alpha | tr '\n' ' ')"
  dybatpho::info "Highest: $(dybatpho::semver_max 1.10.0 1.9.0 2.0.0-rc.1)"

  # A list of tags keeps its `v`, so the result is usable as a tag again.
  dybatpho::info "From a tag list: $(printf 'v1.2.0\nv1.10.0\nv1.9.0\n' | dybatpho::semver_max)"
}

function _main {
  _demo_valid
  _demo_parse
  _demo_compare
  _demo_release_type
  _demo_bump
  _demo_ranges
  _demo_ordering
  dybatpho::success "Semver operations demo complete"
}

_main "$@"
