#!/usr/bin/env bash
# @file release_ops.sh
# @brief Example showing a full release cut from a Git repository
# @description Demonstrates dybatpho::release_commit_type, release_bump_type,
#   release_next_version, release_changelog, release_artifact_name,
#   release_package, release_checksums, release_sign, and dybatpho::git_latest_tag
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
. "${SCRIPTDIR}/../init.sh" --modules release

dybatpho::register_common_handlers

# A throwaway repository, so the demo never touches a real project.
WORKDIR=""
REPO=""

function _setup_repo {
  dybatpho::create_temp WORKDIR "/"
  REPO="${WORKDIR}/project"
  mkdir -p "${REPO}"
  git init -q "${REPO}"
  git -C "${REPO}" config user.name "dybatpho"
  git -C "${REPO}" config user.email "dybatpho@example.com"
  git -C "${REPO}" config commit.gpgsign false
  git -C "${REPO}" config tag.gpgsign false

  _commit "feat: first working version"
  git -C "${REPO}" tag -m v1.0.0 v1.0.0
  _commit "fix(parser): accept an empty input file"
  _commit "feat(api): add the status endpoint"
  _commit "docs: expand the readme"
}

function _commit {
  printf '%s\n' "$1" >> "${REPO}/history"
  git -C "${REPO}" add -A
  git -C "${REPO}" commit -qm "$1"
}

function _demo_version {
  dybatpho::header "DECIDING THE VERSION"
  local previous next

  previous="$(dybatpho::git_latest_tag "${REPO}" 'v*')"
  dybatpho::info "Previous release: ${previous}"
  dybatpho::info "Commits since then call for a $(dybatpho::release_bump_type "${REPO}" "${previous}") release"

  # A `feat` commit moves the minor, so 1.0.0 becomes 1.1.0.
  next="$(dybatpho::release_next_version "${REPO}")"
  dybatpho::info "Next version: ${next}"

  dybatpho::info "How single subjects classify:"
  local subject
  for subject in "feat: a feature" "fix(db): a fix" "feat!: a breaking change" "chore: housekeeping"; do
    dybatpho::print "  $(printf '%-26s' "${subject}") -> $(dybatpho::release_commit_type "${subject}")"
  done
}

function _demo_changelog {
  dybatpho::header "CHANGELOG FROM COMMITS"
  local next
  next="$(dybatpho::release_next_version "${REPO}")"
  # The docs commit is deliberately absent: it is history, not release notes.
  dybatpho::release_changelog "${REPO}" "$(dybatpho::git_latest_tag "${REPO}" 'v*')" HEAD "${next}" >&2
}

function _demo_artifacts {
  dybatpho::header "ARTIFACTS PER PLATFORM"
  local build="${WORKDIR}/build" dist="${WORKDIR}/dist" version="1.1.0" platform artifact
  mkdir -p "${build}"
  printf '#!/bin/sh\necho mytool\n' > "${build}/mytool"

  # In a real build each platform has its own output directory; the same one is
  # reused here because the demo has no cross-compiler.
  for platform in "linux amd64" "linux arm64" "darwin arm64" "windows amd64"; do
    read -r goos goarch <<< "${platform}"
    if [[ "${goos}" == "windows" ]] && ! dybatpho::is command zip; then
      dybatpho::warn "Skipping ${goos}/${goarch}: zip is not installed"
      continue
    fi
    artifact="$(dybatpho::release_package "${build}" "${dist}" mytool "${version}" "${goos}" "${goarch}")"
    dybatpho::print "  $(dybatpho::path_basename "${artifact}")  ($(dybatpho::file_size "${artifact}") bytes)"
  done
}

function _demo_checksums_and_signature {
  dybatpho::header "CHECKSUMS AND SIGNATURE"
  local dist="${WORKDIR}/dist" sums signature

  sums="$(dybatpho::release_checksums "${dist}")"
  dybatpho::info "Wrote $(dybatpho::path_basename "${sums}")"
  dybatpho::show_file "${sums}"
  dybatpho::info "A consumer verifies a download with: sha256sum -c SHA256SUMS"

  # Signing the checksum file covers every artifact, because the checksums bind
  # their contents. This demo uses a stand-in signer so that it needs no key.
  local signer="${WORKDIR}/demo-signer"
  cat > "${signer}" << 'SIGNER'
#!/usr/bin/env bash
printf 'pretend signature of %s\n' "$2" > "$1"
SIGNER
  chmod +x "${signer}"
  signature="$(DYBATPHO_RELEASE_SIGN_CMD="${signer}" dybatpho::release_sign "${sums}")"
  dybatpho::info "Signature: $(dybatpho::path_basename "${signature}")"
  dybatpho::info "With a real key this would be gpg, or minisign through DYBATPHO_RELEASE_SIGN_CMD"
}

function _main {
  _setup_repo
  _demo_version
  _demo_changelog
  _demo_artifacts
  _demo_checksums_and_signature
  dybatpho::success "Release demo complete"
}

_main "$@"
