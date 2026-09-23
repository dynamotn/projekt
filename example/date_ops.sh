#!/usr/bin/env bash
# @file date_ops.sh
# @brief Example showing date and timestamp utilities
# @description Demonstrates dybatpho::date_now, date_today, date_is_valid, date_parse, date_format, date_add_days, and date_diff_days
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=init.sh
. "${SCRIPTDIR}/../init.sh" --modules date

dybatpho::register_common_handlers

function _demo_now {
  dybatpho::header "CURRENT TIME"
  dybatpho::info "Unix timestamp : $(dybatpho::date_now)"
  dybatpho::info "RFC3339-ish    : $(dybatpho::date_now "%Y-%m-%dT%H:%M:%SZ")"
  dybatpho::info "Today          : $(dybatpho::date_today)"
}

function _demo_parse_format {
  dybatpho::header "PARSE / FORMAT"
  local stamp
  stamp=$(dybatpho::date_parse "2024-02-29 12:34:56")
  dybatpho::info "Parsed timestamp: ${stamp}"
  dybatpho::info "Formatted again : $(dybatpho::date_format "${stamp}")"
}

function _demo_math {
  dybatpho::header "DATE MATH"
  dybatpho::info "Add 10 days : $(dybatpho::date_add_days "2024-03-01" 10)"
  dybatpho::info "Subtract 1 day: $(dybatpho::date_add_days "2024-03-01" -1)"
  dybatpho::info "Day diff    : $(dybatpho::date_diff_days "2024-03-01" "2024-03-11")"
}

function _demo_validate {
  dybatpho::header "VALIDATION"
  dybatpho::info "2024-02-29 valid? $(dybatpho::date_is_valid "2024-02-29" && echo yes || echo no)"
  dybatpho::info "2024-02-30 valid? $(dybatpho::date_is_valid "2024-02-30" && echo yes || echo no)"
}

# @description Bound the month a date falls in, which is what a billing period
#   or a report window needs.
function _demo_calendar {
  dybatpho::header "CALENDAR"
  local date
  for date in 2024-02-17 2023-02-17 2024-04-05; do
    dybatpho::print "  ${date}: $(dybatpho::date_month_start "${date}") .. $(dybatpho::date_month_end "${date}")"
  done
  local year
  for year in 1900 2000 2024; do
    if dybatpho::date_is_leap_year "${year}"; then
      dybatpho::print "  ${year} is a leap year, February has $(dybatpho::date_days_in_month "${year}" 2) days"
    else
      dybatpho::print "  ${year} is not, February has $(dybatpho::date_days_in_month "${year}" 2) days"
    fi
  done
}

# @description Shift a date by a span and measure the distance between two,
#   in whichever unit the answer is wanted in.
function _demo_spans {
  dybatpho::header "SPANS"
  dybatpho::print "  2024-02-28 + 1 day   = $(dybatpho::date_add 2024-02-28 1 days '%F')"
  dybatpho::print "  2023-02-28 + 1 day   = $(dybatpho::date_add 2023-02-28 1 days '%F')"
  dybatpho::print "  2024-03-01 - 1 week  = $(dybatpho::date_add 2024-03-01 -1 weeks '%F')"

  local started="2024-01-01 09:00:00" finished="2024-01-02 11:30:00"
  dybatpho::print "  elapsed: $(dybatpho::date_diff "${started}" "${finished}" hours) hours"
  # Truncated toward zero: 26 hours is one day, not two.
  dybatpho::print "  elapsed: $(dybatpho::date_diff "${started}" "${finished}" days) whole days"
  dybatpho::print "  on a clock: $(dybatpho::date_seconds_to_hms "$(dybatpho::date_diff "${started}" "${finished}")")"
}

function _main {
  _demo_now
  _demo_parse_format
  _demo_math
  _demo_validate
  _demo_calendar
  _demo_spans
  dybatpho::success "Date operations demo complete"
}

_main "$@"
