#!/usr/bin/env bash
# @file file_ops.sh
# @brief Example showing file utilities
# @description Demonstrates dybatpho::create_temp, show_file, path_basename, path_dirname, path_extname, path_stem, path_join, path_normalize, path_is_abs, path_has_ext, path_change_ext, path_relative, xdg_config_dir, xdg_cache_dir, xdg_data_dir, xdg_state_dir, create_temp_dir, file_mtime, dir_size, file_is_binary, file_write_atomic, file_replace, file_ensure_line, file_remove_line, file_hash, file_size, file_age_seconds, file_backup, find_up, ensure_dir, and temp cleanup behavior
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=init.sh
. "${SCRIPTDIR}/../init.sh"

dybatpho::register_common_handlers

function _demo_temp_file {
  dybatpho::header "TEMPORARY FILE"

  local TMPFILE
  dybatpho::create_temp TMPFILE ".sh"
  dybatpho::info "Created temp file: ${TMPFILE}"

  cat > "${TMPFILE}" << 'EOF'
# This is a temporary bash snippet
function hello {
  echo "Hello from temp file!"
}
hello
EOF

  dybatpho::info "Contents of temp file:"
  dybatpho::show_file "${TMPFILE}"
  dybatpho::info "Temp file will be deleted automatically when script exits"
}

function _demo_temp_dir {
  dybatpho::header "TEMPORARY DIRECTORY"

  local TMPDIR_VAR
  dybatpho::create_temp TMPDIR_VAR "/"
  dybatpho::info "Created temp directory: ${TMPDIR_VAR}"

  # Create some files inside
  echo "file one" > "${TMPDIR_VAR}/one.txt"
  echo "file two" > "${TMPDIR_VAR}/two.txt"
  mkdir -p "${TMPDIR_VAR}/subdir"
  echo "nested" > "${TMPDIR_VAR}/subdir/three.txt"

  dybatpho::info "Temp dir contents (tree):"
  if command -v tree &> /dev/null; then
    tree "${TMPDIR_VAR}" >&2
  else
    find "${TMPDIR_VAR}" -type f | sort | while IFS= read -r f; do
      dybatpho::print "  ${f}"
    done
  fi

  dybatpho::info "Temp directory will be removed recursively on exit"
}

function _demo_show_file {
  dybatpho::header "SHOW FILE (source of this script)"
  # Show the first 20 lines of this very script
  local TMPFILE
  dybatpho::create_temp TMPFILE ".sh"
  head -20 "${BASH_SOURCE[0]}" > "${TMPFILE}"
  dybatpho::show_file "${TMPFILE}"
}

function _demo_path_parts {
  dybatpho::header "PATH PARTS"
  local path="/tmp/dybatpho/demo/archive.tar.gz"
  dybatpho::info "Path     : ${path}"
  dybatpho::info "Dirname  : $(dybatpho::path_dirname "${path}")"
  dybatpho::info "Basename : $(dybatpho::path_basename "${path}")"
  dybatpho::info "Extname  : $(dybatpho::path_extname "${path}")"
  dybatpho::info "Stem     : $(dybatpho::path_basename "${path}" ".gz")"
  dybatpho::info "Stem 2   : $(dybatpho::path_stem "${path}")"
}

function _demo_path_join {
  dybatpho::header "PATH JOIN"
  dybatpho::info "Joined absolute path: $(dybatpho::path_join "/tmp/" "/dybatpho/" "cache" "data.json")"
  dybatpho::info "Joined relative path: $(dybatpho::path_join "var" "log" "dybatpho")"
}

function _demo_path_normalize {
  dybatpho::header "PATH NORMALIZE"
  dybatpho::info "Normalized absolute path: $(dybatpho::path_normalize "/tmp//dybatpho/./cache/../data.json")"
  dybatpho::info "Normalized relative path: $(dybatpho::path_normalize "var//log/../tmp/./app/")"
}

function _demo_path_checks {
  dybatpho::header "PATH CHECKS / REWRITE"
  local path="/tmp/dybatpho/demo/archive.tar.gz"
  dybatpho::info "Absolute?      : $(dybatpho::path_is_abs "${path}" && echo yes || echo no)"
  dybatpho::info "Has .gz ext?   : $(dybatpho::path_has_ext "${path}" ".gz" && echo yes || echo no)"
  dybatpho::info "Change ext     : $(dybatpho::path_change_ext "${path}" "zip")"
  dybatpho::info "Relative to /tmp: $(dybatpho::path_relative "${path}" "/tmp")"
}

function _demo_file_contents {
  dybatpho::header "FILE CONTENTS"
  # A throwaway copy of a dotfile, so the demo never touches a real one.
  local WORKDIR
  dybatpho::create_temp WORKDIR "/"
  local config="${WORKDIR}/app.conf"

  # `file_write_atomic` takes its content on standard input and swaps the file
  # into place, so a reader never sees a half-written config.
  dybatpho::file_write_atomic "${config}" << 'EOF'
debug = true
port = 8080
EOF
  chmod 640 "${config}"
  dybatpho::info "Wrote ${config}"
  dybatpho::show_file "${config}"

  # Keep a copy before editing, and report where it went.
  local backup
  backup="$(dybatpho::file_backup "${config}")"
  dybatpho::info "Backup kept at $(dybatpho::path_basename "${backup}")"

  # A portable in-place edit: no `sed -i`, whose argument differs on BSD.
  dybatpho::file_replace "${config}" '^debug = true$' 'debug = false'
  dybatpho::info "After replace:"
  dybatpho::show_file "${config}"

  dybatpho::info "Mode survived the rewrite: $(stat -c %a "${config}" 2> /dev/null || stat -f %Lp "${config}")"
}

function _demo_file_lines {
  dybatpho::header "IDEMPOTENT LINES"
  local WORKDIR
  dybatpho::create_temp WORKDIR "/"
  local rc="${WORKDIR}/bashrc"
  printf 'export PATH="${HOME}/bin:${PATH}"\n' > "${rc}"

  # Running the same script twice must not duplicate the line, which is what
  # makes these usable in a dotfiles bootstrap.
  dybatpho::file_ensure_line "${rc}" 'export EDITOR=nvim'
  dybatpho::file_ensure_line "${rc}" 'export EDITOR=nvim'
  dybatpho::info "EDITOR line count after two calls: $(grep -cxF 'export EDITOR=nvim' "${rc}")"

  dybatpho::file_remove_line "${rc}" 'export EDITOR=nvim'
  dybatpho::file_remove_line "${rc}" 'export EDITOR=nvim'
  dybatpho::info "After removing it twice:"
  dybatpho::show_file "${rc}"
}

function _demo_file_metadata {
  dybatpho::header "FILE METADATA"
  local WORKDIR
  dybatpho::create_temp WORKDIR "/"
  local payload="${WORKDIR}/release.txt"
  printf 'dybatpho release payload\n' > "${payload}"

  dybatpho::info "Size    : $(dybatpho::file_size "${payload}") bytes"
  dybatpho::info "SHA-256 : $(dybatpho::file_hash "${payload}")"
  dybatpho::info "MD5     : $(dybatpho::file_hash "${payload}" md5)"
  dybatpho::info "Age     : $(dybatpho::file_age_seconds "${payload}")s since last modification"

  local age
  age="$(dybatpho::file_age_seconds "${payload}")"
  if ((age > 3600)); then
    dybatpho::warn "Cache is stale"
  else
    dybatpho::info "Cache is fresh enough to reuse"
  fi
}

function _demo_dry_run {
  dybatpho::header "DRY RUN"
  local WORKDIR
  dybatpho::create_temp WORKDIR "/"
  local config="${WORKDIR}/guarded.conf"
  printf 'keep = 1\n' > "${config}"

  # Every writer honors DRY_RUN, so a script can be rehearsed before it runs.
  DRY_RUN=true dybatpho::file_replace "${config}" 'keep' 'gone'
  DRY_RUN=true dybatpho::file_ensure_line "${config}" 'added'
  dybatpho::info "File is untouched: $(cat "${config}")"
}

function _demo_find_up {
  dybatpho::header "FIND UP"
  local WORKDIR
  dybatpho::create_temp WORKDIR "/"
  mkdir -p "${WORKDIR}/project/src/deep"
  : > "${WORKDIR}/project/.projectrc"

  # Locating the project root from wherever the caller happens to stand.
  local marker
  if marker="$(dybatpho::find_up ".projectrc" "${WORKDIR}/project/src/deep")"; then
    dybatpho::info "Marker : ${marker}"
    dybatpho::info "Root   : $(dybatpho::path_dirname "${marker}")"
  fi

  if dybatpho::find_up "definitely-not-here" "${WORKDIR}" > /dev/null; then
    dybatpho::warn "Unexpected match"
  else
    dybatpho::info "A missing marker reports failure instead of printing a path"
  fi
}

function _demo_ensure_dir {
  dybatpho::header "ENSURE DIR"
  local WORKDIR
  dybatpho::create_temp WORKDIR "/"

  # `ensure_dir` prints the path, so it composes with the writers directly.
  local cache
  cache="$(dybatpho::ensure_dir "${WORKDIR}/cache/myapp" 700)"
  printf 'last run: ok\n' | dybatpho::file_write_atomic "${cache}/state"
  dybatpho::info "Cache dir : ${cache} (mode $(stat -c %a "${cache}" 2> /dev/null || stat -f %Lp "${cache}"))"
  dybatpho::info "State file: $(cat "${cache}/state")"

  # Calling it again is a no-op, so scripts can call it before every write.
  dybatpho::ensure_dir "${WORKDIR}/cache/myapp" 700 > /dev/null
  dybatpho::info "Second call changed nothing"
}

function _demo_symlinked_dotfile {
  dybatpho::header "SYMLINKED DOTFILE"
  local WORKDIR
  dybatpho::create_temp WORKDIR "/"
  mkdir -p "${WORKDIR}/dotfiles"
  printf 'export EDITOR=vi\n' > "${WORKDIR}/dotfiles/bashrc"
  ln -s "${WORKDIR}/dotfiles/bashrc" "${WORKDIR}/.bashrc"

  # The writers follow the link, so editing the dotfile edits the file in the
  # repository it points at rather than detaching the link from it.
  dybatpho::file_replace "${WORKDIR}/.bashrc" 'EDITOR=vi' 'EDITOR=nvim'
  dybatpho::info "~/.bashrc is still a symlink: $([[ -L "${WORKDIR}/.bashrc" ]] && echo yes || echo no)"
  dybatpho::info "Repository copy now holds  : $(cat "${WORKDIR}/dotfiles/bashrc")"
}

function _demo_xdg {
  dybatpho::header "XDG DIRECTORIES"
  # These only build a path. Pairing them with `ensure_dir`, which prints the
  # directory it made, keeps the whole thing to one line.
  dybatpho::info "config : $(dybatpho::xdg_config_dir myapp)"
  dybatpho::info "cache  : $(dybatpho::xdg_cache_dir myapp)"
  dybatpho::info "data   : $(dybatpho::xdg_data_dir myapp)"
  dybatpho::info "state  : $(dybatpho::xdg_state_dir myapp)"
  dybatpho::info "without an application name: $(dybatpho::xdg_config_dir)"

  # A real script would write into the directory it just made; this one keeps
  # to a temporary root so it never touches the user's home.
  local WORKDIR
  dybatpho::create_temp_dir WORKDIR "xdg"
  local state
  state="$(XDG_STATE_HOME="${WORKDIR}/state" dybatpho::xdg_state_dir myapp)"
  state="$(dybatpho::ensure_dir "${state}" 700)"
  printf 'last-run=%s\n' "$(date +%s)" | dybatpho::file_write_atomic "${state}/run"
  dybatpho::info "Wrote ${state#"${WORKDIR}"/}/run with mode $(stat -c %a "${state}" 2> /dev/null || stat -f %Lp "${state}")"
}

function _demo_inspect {
  dybatpho::header "INSPECTING FILES AND TREES"
  local WORKDIR
  dybatpho::create_temp_dir WORKDIR "inspect"
  mkdir -p "${WORKDIR}/tree/sub"
  head -c 1000 /dev/zero > "${WORKDIR}/tree/a.bin"
  printf 'some text\n' > "${WORKDIR}/tree/sub/notes.txt"
  printf 'binary\000payload' > "${WORKDIR}/tree/sub/blob"

  dybatpho::info "Tree total: $(dybatpho::dir_size "${WORKDIR}/tree") bytes"
  dybatpho::info "One file  : $(dybatpho::file_size "${WORKDIR}/tree/a.bin") bytes"
  dybatpho::info "Modified  : $(dybatpho::file_mtime "${WORKDIR}/tree/sub/notes.txt") (epoch seconds)"

  # Checking before a text rewrite is the point of `file_is_binary`: the
  # substitution below would otherwise mangle the binary file.
  local candidate
  for candidate in "${WORKDIR}/tree/sub/notes.txt" "${WORKDIR}/tree/sub/blob"; do
    if dybatpho::file_is_binary "${candidate}"; then
      dybatpho::warn "  $(dybatpho::path_basename "${candidate}") is binary, leaving it alone"
    else
      dybatpho::file_replace "${candidate}" 'some' 'edited'
      dybatpho::print "  $(dybatpho::path_basename "${candidate}") rewritten: $(cat "${candidate}")"
    fi
  done
}

function _main {
  _demo_temp_file
  _demo_temp_dir
  _demo_show_file
  _demo_path_parts
  _demo_path_join
  _demo_path_normalize
  _demo_path_checks
  _demo_file_contents
  _demo_file_lines
  _demo_file_metadata
  _demo_dry_run
  _demo_find_up
  _demo_ensure_dir
  _demo_symlinked_dotfile
  _demo_xdg
  _demo_inspect
  dybatpho::success "File operations demo complete"
}

_main "$@"
