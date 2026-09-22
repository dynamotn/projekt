#!/usr/bin/env bash
# @file i18n_ops.sh
# @brief Example showing translation and locale-aware formatting
# @description Demonstrates dybatpho::i18n_init, i18n_t, i18n_tn, i18n_number,
#   i18n_currency, i18n_percent, i18n_bytes, i18n_date, i18n_time, i18n_relative,
#   i18n_direction, the bidi helpers, i18n_register_*, i18n_lint, and the
#   i18n_library_text / i18n_library_plural hooks that translate the library's
#   own help, banners, and parser errors
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
# `table` and `text` are not needed by the i18n module; this demo asks for them
# only to lay its output out. Aligning columns that contain Japanese or Arabic
# needs a display width rather than a character count, which is what
# `dybatpho::table_align` measures and a plain `printf '%-20s'` does not.
# shellcheck source=init.sh
. "${SCRIPTDIR}/../init.sh" --modules i18n table text cli

dybatpho::register_common_handlers

# Everything below is pinned so the output is the same on every machine and in
# CI: a fixed timestamp, a fixed timezone, and an explicit locale on every call.
readonly FIXED_EPOCH=1709210096 # 2024-02-29 12:34:56 UTC
export DYBATPHO_DATE_TIMEZONE=UTC
readonly LOCALES=(en de fr vi ja hi ar)

WORKDIR=""

function _setup {
  # The directory and everything in it is removed when this shell exits.
  dybatpho::create_temp_dir WORKDIR i18n-demo
  mkdir -p "${WORKDIR}/locale/vi_VN/LC_MESSAGES" "${WORKDIR}/locale/en/LC_MESSAGES"

  cat > "${WORKDIR}/locale/en/LC_MESSAGES/messages.msg" << 'CATALOG'
# Reference catalog. The keys here are what every other locale is checked against.
deploy.start = Deploying {app} to {env}
deploy.files[one] = {count} file uploaded
deploy.files[other] = {count} files uploaded
deploy.done = Deployment finished
CATALOG

  cat > "${WORKDIR}/locale/vi_VN/LC_MESSAGES/messages.msg" << 'CATALOG'
@locale = vi_VN
deploy.start = Đang triển khai {app} lên {env}
deploy.files[other] = Đã tải lên {count} tệp
# `deploy.done` is deliberately left untranslated so i18n_lint has something
# to report at the end of this demo.

# dybatpho's own interface. The keys below are the library's, not this script's:
# translating them is what stops a CLI reading half in Vietnamese and half in
# English. A short key is used wherever the English carries no value, and the
# English sentence itself is the key for a diagnostic that carries none either.
cli.heading_usage = Cách dùng:
cli.heading_options = Tùy chọn:
cli.heading_commands = Lệnh:
cli.placeholder_options = [TÙY-CHỌN]
cli.placeholder_args = [ĐỐI-SỐ]...
cli.show_help = Hiện trợ giúp này
cli.unrecognized_option = Tùy chọn không hợp lệ: {option}
cli.did_you_mean = . Ý bạn là '{suggestion}'?
cli.args_exact = Cần đúng {count} đối số, nhận được {got}
logging.done = XONG:
"Deployment finished" = Đã triển khai xong
CATALOG

  export DYBATPHO_I18N_PATH="${WORKDIR}/locale"
}

function _demo_translate {
  dybatpho::header "TRANSLATING MESSAGES"
  dybatpho::info "The same script, run under two locales"
  local locale
  for locale in en vi_VN; do
    # i18n_init resolves the locale and loads its catalogs once, in this shell.
    # Doing it here rather than inside the substitutions below matters: each
    # "$( )" is a subshell, so a catalog loaded inside one would be thrown away.
    dybatpho::i18n_reset
    dybatpho::i18n_init "${locale}"
    dybatpho::print "  [${locale}] $(dybatpho::i18n_t deploy.start app=api env=production)"
    dybatpho::print "  [${locale}] $(dybatpho::i18n_tn deploy.files 1)"
    dybatpho::print "  [${locale}] $(dybatpho::i18n_tn deploy.files 1240)"
    dybatpho::print "  [${locale}] $(dybatpho::i18n_t deploy.done)"
  done
  dybatpho::info "Vietnamese has one plural form, so 1 and 1,240 read alike;"
  dybatpho::info "'deploy.done' has no Vietnamese entry, so it falls back to its key"
}

function _demo_plurals {
  dybatpho::header "PLURAL RULES"
  dybatpho::info "How many forms a language has, and which one a count takes"
  local language count line
  for language in en vi ru pl ar; do
    line=""
    for count in 0 1 2 5 21; do
      line+="$(printf '%-6s' "$(dybatpho::i18n_plural_form "${count}" "${language}")")"
    done
    dybatpho::print "  $(printf '%-4s' "${language}") ${line}"
  done
  dybatpho::print "       $(printf '%-6s' 0 1 2 5 21)"
  dybatpho::info "Russian and Polish differ at 21, which is the mistake a hand-written"
  dybatpho::info "'count == 1' check always makes"
}

function _demo_numbers {
  dybatpho::header "NUMBERS"
  local locale
  for locale in "${LOCALES[@]}"; do
    dybatpho::print "  $(printf '%-4s' "${locale}") $(dybatpho::i18n_number 1234567.891 2 "${locale}")"
  done
  dybatpho::info "Hindi groups by lakh rather than by thousand, which is why the"
  dybatpho::info "group size is data and not a hard-coded three"
  dybatpho::print "  A value wider than the machine word, formatted exactly:"
  dybatpho::print "    $(dybatpho::i18n_number 123456789012345678901234567890 0 en)"
}

function _demo_currency {
  dybatpho::header "CURRENCY"
  dybatpho::info "How many decimals to show belongs to the currency;"
  dybatpho::info "where the symbol goes and how digits group belongs to the locale"
  local code locale rows="code|en|de|fr|vi|ja"
  for code in USD EUR JPY VND BHD; do
    rows+=$'\n'"${code}"
    for locale in en de fr vi ja; do
      rows+="|$(dybatpho::i18n_currency 1234.5 "${code}" "${locale}")"
    done
  done
  dybatpho::table_align "${rows}" "|" "" 2 | dybatpho::text_indent - "  "
  dybatpho::print ""
  dybatpho::print "  Negative, default:    $(dybatpho::i18n_currency -1234.5 USD en)"
  dybatpho::print "  Negative, accounting: $(DYBATPHO_I18N_CURRENCY_NEGATIVE=parens dybatpho::i18n_currency -1234.5 USD en)"
  dybatpho::info "The minus goes outside the symbol; a naive version prints \$-1,234.50"
  # An unknown currency is a data gap rather than a reason to stop a report.
  dybatpho::print "  Unknown code:         $(dybatpho::i18n_currency 1234.5 XPF en 2> /dev/null)"
}

function _demo_percent_and_bytes {
  dybatpho::header "PERCENTAGES AND SIZES"
  local locale
  for locale in en fr tr; do
    dybatpho::print "  $(printf '%-4s' "${locale}") $(dybatpho::i18n_percent 42.5 1 "${locale}")"
  done
  dybatpho::info "Turkish writes the sign before the number"

  # A real file, so the byte count comes from the filesystem rather than a
  # literal, showing how i18n_bytes pairs with dybatpho::file_size.
  local sample="${WORKDIR}/sample.bin"
  head -c 1500000 /dev/zero > "${sample}"
  local size
  size="$(dybatpho::file_size "${sample}")"
  dybatpho::print ""
  dybatpho::print "  dybatpho::file_size reports ${size} bytes, which reads as:"
  dybatpho::print "    IEC, en: $(dybatpho::i18n_bytes "${size}" iec en)"
  dybatpho::print "    SI,  en: $(dybatpho::i18n_bytes "${size}" si en)"
  dybatpho::print "    IEC, de: $(dybatpho::i18n_bytes "${size}" iec de)"
  dybatpho::info "The digits are localized; the unit symbols are not, because every"
  dybatpho::info "other tool on the machine prints them the same way"
}

function _demo_dates {
  dybatpho::header "DATES AND TIMES"
  local locale rows="locale|short|full|time"
  for locale in "${LOCALES[@]}"; do
    rows+=$'\n'"${locale}"
    rows+="|$(dybatpho::i18n_date "${FIXED_EPOCH}" short "${locale}")"
    rows+="|$(dybatpho::i18n_date "${FIXED_EPOCH}" full "${locale}")"
    rows+="|$(dybatpho::i18n_time "${FIXED_EPOCH}" short "${locale}")"
  done
  dybatpho::table_align "${rows}" "|" "" 2 | dybatpho::text_indent - "  "
  dybatpho::info "Month names come from this module, never from LC_TIME: a host that"
  dybatpho::info "has not generated a locale answers in English instead of failing"
  dybatpho::print ""
  dybatpho::print "  Same instant, two timezones:"
  dybatpho::print "    UTC:            $(dybatpho::i18n_datetime "${FIXED_EPOCH}" medium en)"
  dybatpho::print "    Asia/Ho_Chi_Minh: $(DYBATPHO_DATE_TIMEZONE=Asia/Ho_Chi_Minh dybatpho::i18n_datetime "${FIXED_EPOCH}" medium en)"
}

function _demo_relative {
  dybatpho::header "RELATIVE TIME"
  dybatpho::i18n_reset
  dybatpho::i18n_init en
  local offset
  for offset in 10 300 7200 259200 1209600 5259600; do
    dybatpho::print "  $(printf '%-10s' "-${offset}s") $(dybatpho::i18n_relative $((FIXED_EPOCH - offset)) "${FIXED_EPOCH}")"
  done
  dybatpho::print "  $(printf '%-10s' "+7200s") $(dybatpho::i18n_relative $((FIXED_EPOCH + 7200)) "${FIXED_EPOCH}")"
  dybatpho::info "The reference time is passed in, which is what keeps this output"
  dybatpho::info "reproducible instead of depending on when the script runs"
}

function _demo_direction {
  dybatpho::header "TEXT DIRECTION"
  local locale
  for locale in en de ja ar he fa; do
    dybatpho::print "  $(printf '%-4s' "${locale}") $(dybatpho::i18n_direction "${locale}")"
  done
  local product="مرحبا"
  local wrapped
  wrapped="$(dybatpho::i18n_bidi_isolate "${product}")"
  dybatpho::print ""
  dybatpho::print "  Spliced raw:      Package ${product} installed"
  dybatpho::print "  Spliced isolated: Package ${wrapped} installed"
  dybatpho::print "  Stripped again:   $(dybatpho::i18n_bidi_strip "${wrapped}")"
  dybatpho::info "Isolating is for display only. The markers are invisible but real,"
  dybatpho::info "so wrapped text no longer compares equal to what it was wrapped from"
}

function _demo_register {
  dybatpho::header "ADDING A LOCALE WITHOUT PATCHING THE MODULE"
  dybatpho::i18n_register_number sv " " "," 3
  dybatpho::i18n_register_currency_layout sv '# ¤'
  dybatpho::i18n_register_currency SEK "kr" 2
  dybatpho::i18n_register_names sv months \
    "januari,februari,mars,april,maj,juni,juli,augusti,september,oktober,november,december"
  dybatpho::i18n_register_names sv weekdays \
    "måndag,tisdag,onsdag,torsdag,fredag,lördag,söndag"
  dybatpho::i18n_register_date sv date_long "d MMMM yyyy"
  dybatpho::print "  number:   $(dybatpho::i18n_number 1234567.89 2 sv)"
  dybatpho::print "  currency: $(dybatpho::i18n_currency 1234.5 SEK sv)"
  dybatpho::print "  date:     $(dybatpho::i18n_date "${FIXED_EPOCH}" long sv)"
  dybatpho::info "Six calls in the caller's own script, no edit to src/i18n.sh"
}

function _demo_tooling {
  dybatpho::header "KEEPING CATALOGS HONEST"
  local source="${WORKDIR}/app.sh"
  cat > "${source}" << 'SOURCE'
dybatpho::i18n_t deploy.start app=api env=production
dybatpho::i18n_tn deploy.files 12
dybatpho::i18n_t "deploy.${stage}.note"
SOURCE
  dybatpho::info "Extracting the keys a source tree refers to"
  # The dynamic key is reported on stderr rather than guessed at, which is the
  # point: it names the call site the tooling cannot see.
  dybatpho::i18n_extract --locale en "${source}" 2> "${WORKDIR}/extract.err" \
    | dybatpho::text_indent - "  " || true
  dybatpho::print "  stderr said:"
  dybatpho::text_indent "$(cat "${WORKDIR}/extract.err")" "    " || true

  dybatpho::info "Checking a translation against the reference locale"
  dybatpho::i18n_reset
  dybatpho::i18n_load en "${WORKDIR}/locale/en/LC_MESSAGES/messages.msg"
  dybatpho::i18n_load vi_VN "${WORKDIR}/locale/vi_VN/LC_MESSAGES/messages.msg"
  if dybatpho::i18n_lint --reference en vi_VN | dybatpho::text_indent - "  "; then
    dybatpho::print "  nothing to report"
  fi
  dybatpho::info "'missing' is the untranslated key; a dropped {placeholder} would"
  dybatpho::info "be reported too, which is the failure nothing else catches"
}

# Run one call with the library's own text routed through the catalog. The
# subshell is the point: the rest of the demo keeps printing in English, and
# nothing set here reaches the caller. ShellCheck reads that confinement as a
# lost assignment, which is exactly the behavior being relied on.
# shellcheck disable=SC2030,SC2031
function _translated {
  (
    export DYBATPHO_I18N_TRANSLATE_LIBRARY=true
    "$@"
  )
}

function _demo_library_ui {
  dybatpho::header "THE LIBRARY'S OWN INTERFACE"
  dybatpho::info "Translating your strings still leaves dybatpho's half of the"
  dybatpho::info "screen in English: 'Usage:', 'Options:', 'Unrecognized option'."
  dybatpho::info "DYBATPHO_I18N_TRANSLATE_LIBRARY routes those through the catalog"
  dybatpho::info "too. It is off by default, so nothing changes until it is asked for."

  dybatpho::i18n_reset
  dybatpho::i18n_init vi_VN

  dybatpho::print ""
  dybatpho::print "  Generated help, before and after:"
  (dybatpho::generate_help _demo_spec) | dybatpho::text_indent - "    "
  dybatpho::print ""
  _translated dybatpho::generate_help _demo_spec | dybatpho::text_indent - "    "

  dybatpho::print ""
  dybatpho::print "  A rejected switch. The English names the switch, so it cannot"
  dybatpho::print "  be its own message id; the call site passes a key instead:"
  (dybatpho::generate_from_spec _demo_spec --dst /tmp) 2>&1 \
    | dybatpho::text_indent - "    " || true
  _translated dybatpho::generate_from_spec _demo_spec --dst /tmp 2>&1 \
    | dybatpho::text_indent - "    " || true

  dybatpho::print ""
  dybatpho::print "  A banner, whose label and message are translated separately:"
  _translated dybatpho::success "Deployment finished"
  dybatpho::info "The banner re-measures its border around the translated text,"
  dybatpho::info "which is why the box is not the width it was in English"
}

# The spec whose help and errors the section above renders. It is deliberately
# plain: everything interesting in that output belongs to the library.
function _demo_spec {
  dybatpho::opts::setup "Deploy an application" ARGS action:"true"
  dybatpho::opts::param "Where to deploy" DEST -d --dest
}

function _main {
  _setup
  _demo_translate
  _demo_plurals
  _demo_numbers
  _demo_currency
  _demo_percent_and_bytes
  _demo_dates
  _demo_relative
  _demo_direction
  _demo_register
  _demo_tooling
  _demo_library_ui
  dybatpho::success "Internationalization and localization demo complete"
}

_main "$@"
