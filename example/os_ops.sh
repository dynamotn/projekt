#!/usr/bin/env bash
# @file os_ops.sh
# @brief Example showing platform, host, and environment detection.
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
. "${SCRIPTDIR}/../init.sh"

dybatpho::header "PLATFORM"
dybatpho::print "platform: $(dybatpho::platform)"
dybatpho::print "architecture: $(dybatpho::goarch)"
dybatpho::print "release artifact: mytool_$(dybatpho::goos)_$(dybatpho::goarch).tar.gz"

if dybatpho::is_macos; then
  dybatpho::info "Running on macOS"
elif dybatpho::is_linux; then
  dybatpho::info "Running on Linux"
elif dybatpho::is_windows; then
  dybatpho::info "Running on a Windows-compatible shell"
fi

dybatpho::header "DISTRIBUTION"
dybatpho::print "distribution: $(dybatpho::distro)"
dybatpho::print "version: $(dybatpho::distro_version || printf 'unknown')"
dybatpho::print "kernel: $(dybatpho::kernel_version)"
# The whole family is what usually decides a package name, and a derivative
# reports its own `ID`, so `ID_LIKE` is read directly when it exists.
if family="$(dybatpho::os_release ID_LIKE)"; then
  dybatpho::print "family: ${family}"
fi

dybatpho::header "HOST"
dybatpho::print "host: $(dybatpho::hostname)"
dybatpho::print "user: $(dybatpho::user)"
if dybatpho::is_root; then
  dybatpho::warn "Running as root"
else
  dybatpho::info "Running unprivileged"
fi

dybatpho::header "CAPACITY"
# A host that cannot answer gets a conservative default rather than a failure.
jobs="$(dybatpho::cpu_count || printf '4')"
dybatpho::print "worker jobs: ${jobs}"
dybatpho::print "terminal: $(dybatpho::terminal_width)x$(dybatpho::terminal_height)"
if dybatpho::is_tty stdout; then
  dybatpho::info "Output is a terminal, a progress line is worth rendering"
else
  dybatpho::info "Output is redirected, printing plain lines instead"
fi

dybatpho::header "ENVIRONMENT"
for fact in is_container is_wsl is_ci; do
  if "dybatpho::${fact}"; then
    dybatpho::print "${fact}: yes"
  else
    dybatpho::print "${fact}: no"
  fi
done

if tool_path="$(dybatpho::command_path git gh curl)"; then
  dybatpho::print "available tool: ${tool_path}"
else
  dybatpho::warn "None of git, gh, or curl is installed"
fi

dybatpho::header "TOOL VERSIONS"
# Asking a command what version it is, rather than only whether it exists. Not
# every tool answers in a way that can be read, so the failure is expected and
# handled rather than fatal.
for tool in bash git tar; do
  if ! dybatpho::is command "${tool}"; then
    dybatpho::print "${tool}: not installed"
  elif tool_version="$(dybatpho::command_version "${tool}")"; then
    dybatpho::print "${tool}: ${tool_version}"
  else
    dybatpho::print "${tool}: installed, version could not be read"
  fi
done
