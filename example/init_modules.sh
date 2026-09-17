#!/usr/bin/env bash
# @file init_modules.sh
# @brief Example showing how to bootstrap dybatpho with a subset of modules.
#
# A release script only needs a handful of modules, so it asks for them by name
# instead of paying for the whole library. Anything else it turns out to need
# later is loaded on demand.
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
. "${SCRIPTDIR}/../init.sh" --modules git semver

dybatpho::header "REQUESTED MODULE SET"
dybatpho::print "requested: git semver"
dybatpho::print "loaded:    $(dybatpho::module_list loaded | tr '\n' ' ')"
dybatpho::info "The core modules come along with every module set"

# A module set is a contract the script can check before it relies on one.
if dybatpho::module_loaded semver; then
  dybatpho::print "next minor release: $(dybatpho::semver_bump "1.4.2" minor)"
fi

if ! dybatpho::module_loaded network; then
  dybatpho::info "network is not loaded, so curl helpers stay out of this shell"
fi

dybatpho::header "LOADING A MODULE ON DEMAND"
# The release notes turn out to need a table, and text depends on table, so
# asking for `text` brings both in.
dybatpho::load text
dybatpho::print "loaded now: $(dybatpho::module_list loaded | tr '\n' ' ')"
dybatpho::info "text pulled in its table dependency automatically"

dybatpho::print "$(dybatpho::text_indent "release notes are indented by two spaces")"

dybatpho::header "AVAILABLE MODULES"
dybatpho::print "core:     $(dybatpho::module_list core | tr '\n' ' ')"
dybatpho::print "optional: $(dybatpho::module_list optional | tr '\n' ' ')"

dybatpho::success "Module demo complete"
