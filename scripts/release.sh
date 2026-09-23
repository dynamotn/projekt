#!/usr/bin/env bash
# @file release.sh
# @brief Cut a release of projekt from the current repository
# @description
#   Decides the next version from the Conventional Commits made since the last
#   tag, writes the changelog entry, then commits, tags and pushes. The actual
#   build and the GitHub release are left to goreleaser, which the `release`
#   workflow runs when the tag lands, so nothing here needs a forge token.
#
#   Every state-changing step goes through `dybatpho::dry_run`, so `--dry-run`
#   shows the whole plan without touching the repository or the remote.
#
# Usage examples:
#   scripts/release.sh --dry-run
#   scripts/release.sh --bump minor
#   scripts/release.sh --version 1.4.0 --yes
set -euo pipefail

# The library lives in the dotfiles checkout by default; point DYBATPHO_DIR
# somewhere else to use another copy, such as a vendored one in CI.
DYBATPHO_PATH="${DYBATPHO_DIR:-${HOME}/Dotfiles/scripts/lib/dybatpho}"
if [[ ! -r "${DYBATPHO_PATH}/init.sh" ]]; then
  printf 'dybatpho not found at %s; set DYBATPHO_DIR to its checkout\n' \
    "${DYBATPHO_PATH}" >&2
  exit 1
fi
# shellcheck source=/dev/null
. "${DYBATPHO_PATH}/init.sh" --modules release safety cli

dybatpho::register_common_handlers

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHANGELOG="${REPO_ROOT}/CHANGELOG.md"
TAG_PATTERN="${DYBATPHO_RELEASE_TAG_PATTERN:-v*}"

# ---------------------------------------------------------------------------
# Steps
# ---------------------------------------------------------------------------

# Refuse to release from a tree that doesn't match what the tag will claim.
function _preflight {
  dybatpho::require git
  dybatpho::require go
  dybatpho::require make

  local branch default_branch
  branch="$(dybatpho::git_branch "${REPO_ROOT}")"
  default_branch="$(dybatpho::git_default_branch "${REPO_ROOT}")"
  if [[ "${branch}" != "${default_branch}" ]]; then
    dybatpho::confirm \
      "On branch ${branch}, not ${default_branch}. Release from it anyway?" \
      || dybatpho::die "Switch to ${default_branch} before releasing"
  fi

  dybatpho::git_is_clean "${REPO_ROOT}" \
    || dybatpho::die "Worktree has changes; commit or stash them first"

  dybatpho::git_has_remote "${REMOTE}" "${REPO_ROOT}" \
    || dybatpho::die "No remote named ${REMOTE}"
}

# Sets a variable rather than printing, so that a rejected version stops the
# script itself instead of only the command substitution that would read it.
# @set RELEASE_VERSION Version to release, without a leading `v`
function _resolve_version {
  if [[ -n "${VERSION:-}" ]]; then
    RELEASE_VERSION="${VERSION#v}"
    dybatpho::semver_valid "${RELEASE_VERSION}" \
      || dybatpho::die "Not a valid semantic version: ${VERSION}"
  elif [[ -n "${BUMP:-}" ]]; then
    [[ -n "${PREVIOUS_TAG}" ]] \
      || dybatpho::die "No ${TAG_PATTERN} tag to bump from; pass --version"
    RELEASE_VERSION="$(dybatpho::semver_bump "${PREVIOUS_TAG#v}" "${BUMP}")"
  else
    # No commit that calls for a release leaves this empty, which is a stop,
    # not a failure: there is simply nothing to ship.
    RELEASE_VERSION="$(dybatpho::release_next_version "${REPO_ROOT}" || true)"
    [[ -n "${RELEASE_VERSION}" ]] \
      || dybatpho::die "No feat/fix/perf commit since ${PREVIOUS_TAG:-the first commit}; nothing to release"
  fi
}

# @arg $1 string Version being released
function _check_tag_free {
  local tag="v$1"
  if git -C "${REPO_ROOT}" rev-parse -q --verify "refs/tags/${tag}" > /dev/null; then
    dybatpho::die "Tag ${tag} already exists"
  fi
}

function _run_gates {
  if dybatpho::is true "${SKIP_CHECKS:-false}"; then
    dybatpho::warn "Skipping lint and tests"
    return 0
  fi
  dybatpho::info "Running lint"
  dybatpho::dry_run make -C "${REPO_ROOT}" lint
  dybatpho::info "Running tests"
  dybatpho::dry_run make -C "${REPO_ROOT}" test
}

# Put the new section directly under the changelog title, so the file stays
# newest-first the way Keep a Changelog describes.
# @arg $1 string Version being released
function _write_changelog {
  local version="$1" entry temp_dir temp_file
  entry="$(dybatpho::release_changelog \
    "${REPO_ROOT}" "${PREVIOUS_TAG}" HEAD "${version}")"

  dybatpho::create_temp temp_dir "/"
  temp_file="${temp_dir}/CHANGELOG.md"
  {
    printf '# Changelog\n\n'
    printf '%s\n\n' "${entry}"
    if [[ -f "${CHANGELOG}" ]]; then
      # Drop the old title and the blank line after it; the rest is history.
      sed -e '1{/^# Changelog$/d}' -e '1{/^$/d}' "${CHANGELOG}"
    fi
  } > "${temp_file}"

  dybatpho::header "CHANGELOG ENTRY"
  dybatpho::print "${entry}"

  dybatpho::dry_run cp "${temp_file}" "${CHANGELOG}"
}

# @arg $1 string Version being released
function _publish {
  local version="$1" tag="v$1" branch
  branch="$(dybatpho::git_branch "${REPO_ROOT}")"

  dybatpho::dry_run git -C "${REPO_ROOT}" add CHANGELOG.md
  dybatpho::dry_run git -C "${REPO_ROOT}" commit -m "chore(release): ${tag}"
  dybatpho::dry_run git -C "${REPO_ROOT}" tag -a "${tag}" -m "Release ${tag}"

  if dybatpho::is true "${NO_PUSH:-false}"; then
    dybatpho::warn "Not pushing; run: git push ${REMOTE} ${branch} && git push ${REMOTE} ${tag}"
    return 0
  fi
  dybatpho::dry_run git -C "${REPO_ROOT}" push "${REMOTE}" "${branch}"
  dybatpho::dry_run git -C "${REPO_ROOT}" push "${REMOTE}" "${tag}"
  dybatpho::info "Pushed ${tag}; the release workflow builds and publishes it"
}

function _release {
  REMOTE="${REMOTE:-origin}"
  if dybatpho::is true "${DRY_RUN:-false}"; then
    dybatpho::warn "Dry run: nothing will be changed"
  fi

  _preflight

  PREVIOUS_TAG="$(dybatpho::git_latest_tag "${REPO_ROOT}" "${TAG_PATTERN}" || true)"
  dybatpho::info "Previous release: ${PREVIOUS_TAG:-none}"

  local version
  _resolve_version
  version="${RELEASE_VERSION}"
  _check_tag_free "${version}"
  dybatpho::info "Releasing v${version}"

  _run_gates
  _write_changelog "${version}"

  dybatpho::confirm "Commit, tag and push v${version}?" yes \
    || dybatpho::die "Release aborted"

  _publish "${version}"
  dybatpho::success "Released v${version}"
}

# ---------------------------------------------------------------------------
# Spec
# ---------------------------------------------------------------------------

function _spec {
  dybatpho::opts::setup \
    "Cut a release of projekt: version, changelog, tag and push" \
    ARGS action:"_release"

  dybatpho::opts::param "Version to release, instead of deriving one" VERSION -v --version
  dybatpho::opts::param "Bump the last tag by this part (major|minor|patch)" BUMP -b --bump
  dybatpho::opts::param "Remote to push to" REMOTE -r --remote init:="origin"
  dybatpho::opts::flag "Show every step without changing anything" DRY_RUN -n --dry-run
  dybatpho::opts::flag "Skip lint and tests" SKIP_CHECKS --skip-checks
  dybatpho::opts::flag "Commit and tag, but don't push" NO_PUSH --no-push
  dybatpho::opts::flag "Answer yes to every prompt" DYBATPHO_FORCE -y --yes

  dybatpho::opts::disp "Show help" -h --help action:"dybatpho::generate_help _spec"
}

dybatpho::generate_from_spec _spec "$@"
