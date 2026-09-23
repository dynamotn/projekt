#!/usr/bin/env bash
# @file math_ops.sh
# @brief Example showing exact decimal arithmetic
# @description Demonstrates dybatpho::math_add, math_sub, math_mul, math_div,
#   math_mod, math_pow, math_abs, math_neg, math_compare, math_gt, math_lt,
#   math_eq, math_round, math_floor, math_ceil, math_trunc, math_min, math_max,
#   math_sum, math_avg, math_clamp, math_percent, math_gcd, math_lcm,
#   math_random, math_is_number and math_is_integer, by pricing an invoice and
#   summarizing a batch of timings.
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=init.sh
. "${SCRIPTDIR}/../init.sh" --modules math

dybatpho::register_common_handlers

function _demo_exactness {
  dybatpho::header "WHY NOT awk OR bc"
  # Binary floating point cannot hold 0.1, so `awk` answers 0.30000000000000004
  # and a total built from cents slowly drifts. Digit arithmetic does not.
  dybatpho::info "0.1 + 0.2       = $(dybatpho::math_add 0.1 0.2)"
  dybatpho::info "1.1 - 1.0       = $(dybatpho::math_sub 1.1 1.0)"
  # `$(( ))` would silently wrap past 2^63 here.
  dybatpho::info "99999999999 ^ 2 = $(dybatpho::math_pow 99999999999 2)"
  dybatpho::info "2.50 == 2.5     : $(dybatpho::math_eq 2.50 2.5 && echo yes || echo no)"
}

function _demo_invoice {
  dybatpho::header "AN INVOICE, TO THE CENT"
  local -a prices=(19.99 4.50 129.00)
  local -a quantities=(3 2 1)
  local index subtotal="0" line

  # Results are canonical, so a line worth 9.00 prints as 9. Putting the cents
  # back is display work, which is what `dybatpho::i18n_currency` is for.
  for index in "${!prices[@]}"; do
    line="$(dybatpho::math_mul "${prices[index]}" "${quantities[index]}")"
    subtotal="$(dybatpho::math_add "${subtotal}" "${line}")"
    dybatpho::print "  $(printf '%-11s' "${quantities[index]} x ${prices[index]}") = ${line}"
  done

  # A discount and a tax rate are ratios; rounding happens once, at the end,
  # where money is actually charged.
  local discount tax total
  discount="$(dybatpho::math_mul "${subtotal}" 0.10)"
  tax="$(dybatpho::math_mul "$(dybatpho::math_sub "${subtotal}" "${discount}")" 0.0825)"
  total="$(dybatpho::math_round \
    "$(dybatpho::math_add "$(dybatpho::math_sub "${subtotal}" "${discount}")" "${tax}")" 2)"

  dybatpho::info "Subtotal        : ${subtotal}"
  dybatpho::info "Discount (10%)  : -${discount}"
  dybatpho::info "Tax (8.25%)     : ${tax}"
  dybatpho::success "Total charged   : ${total}"

  # The share each line takes of the bill, which is what a report shows.
  dybatpho::info "First line is $(dybatpho::math_percent \
    "$(dybatpho::math_mul "${prices[0]}" "${quantities[0]}")" "${subtotal}" 1)% of the subtotal"
}

function _demo_statistics {
  dybatpho::header "SUMMARIZING A BATCH"
  local -a durations=(0.482 1.205 0.997 2.310 0.874)
  dybatpho::info "Samples : ${durations[*]}"
  dybatpho::info "Count   : ${#durations[@]}"
  dybatpho::info "Total   : $(dybatpho::math_sum "${durations[@]}")"
  dybatpho::info "Fastest : $(dybatpho::math_min "${durations[@]}")"
  dybatpho::info "Slowest : $(dybatpho::math_max "${durations[@]}")"
  dybatpho::info "Mean    : $(DYBATPHO_MATH_SCALE=3 dybatpho::math_avg "${durations[@]}")"

  # A list arrives on a pipe as often as in an array.
  dybatpho::info "From a pipe: $(printf '%s\n' "${durations[@]}" | dybatpho::math_sum)"

  local budget="1.000"
  local slowest
  slowest="$(dybatpho::math_max "${durations[@]}")"
  if dybatpho::math_gt "${slowest}" "${budget}"; then
    dybatpho::warn "Slowest sample ${slowest}s is over the ${budget}s budget"
  else
    dybatpho::success "Every sample is inside the ${budget}s budget" # kcov(skip)
  fi
}

function _demo_rounding {
  dybatpho::header "ROUNDING THAT SAYS WHAT IT DOES"
  local value="2.665"
  dybatpho::info "Value  : ${value}"
  dybatpho::print "  round  (2 digits): $(dybatpho::math_round "${value}" 2)"
  dybatpho::print "  round  (0 digits): $(dybatpho::math_round "${value}")"
  dybatpho::print "  floor            : $(dybatpho::math_floor "${value}")"
  dybatpho::print "  ceil             : $(dybatpho::math_ceil "${value}")"
  dybatpho::print "  trunc            : $(dybatpho::math_trunc "${value}")"
  dybatpho::print "  the same, below zero:"
  local negative="-2.665"
  dybatpho::print "    round: $(dybatpho::math_round "${negative}") floor: $(dybatpho::math_floor "${negative}") ceil: $(dybatpho::math_ceil "${negative}") trunc: $(dybatpho::math_trunc "${negative}")"
  dybatpho::print "  abs / neg        : $(dybatpho::math_abs "${negative}") / $(dybatpho::math_neg 2.665)"
}

function _demo_progress {
  dybatpho::header "A PROGRESS READOUT"
  local done_count=7 total_count=9 percent bar_width filled index bar=""
  percent="$(dybatpho::math_percent "${done_count}" "${total_count}" 1)"
  bar_width=20
  # Clamping keeps a rounding error or a miscounted job from drawing a bar that
  # is longer than the bar.
  filled="$(dybatpho::math_clamp \
    "$(dybatpho::math_round "$(dybatpho::math_div \
      "$(dybatpho::math_mul "${done_count}" "${bar_width}")" "${total_count}")")" \
    0 "${bar_width}")"
  for ((index = 0; index < bar_width; index++)); do
    if ((index < filled)); then
      bar="${bar}#"
    else
      bar="${bar}."
    fi
  done
  dybatpho::info "[${bar}] ${percent}% (${done_count}/${total_count})"
}

function _demo_whole_numbers {
  dybatpho::header "WHOLE-NUMBER HELPERS"
  dybatpho::info "17 mod 5        : $(dybatpho::math_mod 17 5)"
  dybatpho::info "-17 mod 5       : $(dybatpho::math_mod -17 5)"
  dybatpho::info "gcd(24, 36, 60) : $(dybatpho::math_gcd 24 36 60)"
  dybatpho::info "lcm(4, 6)       : $(dybatpho::math_lcm 4 6)"
  # An aspect ratio is a gcd: 1920x1080 reduces to 16:9.
  local width=1920 height=1080 divisor
  divisor="$(dybatpho::math_gcd "${width}" "${height}")"
  dybatpho::info "${width}x${height} is $(dybatpho::math_div "${width}" "${divisor}" 0):$(dybatpho::math_div "${height}" "${divisor}" 0)"

  # Retry jitter, without the bias `$((RANDOM % 5))` would introduce.
  local attempt
  for attempt in 1 2 3; do
    dybatpho::print "  attempt ${attempt}: would sleep $(dybatpho::math_random 1 5)s"
  done
}

function _demo_validation {
  dybatpho::header "CHECKING INPUT BEFORE COMPUTING"
  local candidate
  for candidate in "42" "-3.5" "1e3" "1,5" "" "2.00"; do
    if ! dybatpho::math_is_number "${candidate}"; then
      dybatpho::warn "  '${candidate}' is not a number this module accepts"
      continue
    fi
    if dybatpho::math_is_integer "${candidate}"; then
      dybatpho::print "  '${candidate}' is a whole number"
    else
      dybatpho::print "  '${candidate}' is a decimal"
    fi
  done

  # The shape a real guard takes: validate, then compute.
  local reported="12.75"
  dybatpho::math_is_number "${reported}" \
    || dybatpho::die "Refusing to bill against '${reported}'"
  dybatpho::success "Ranked by value: $(dybatpho::math_compare "${reported}" 12.8) means ${reported} < 12.8"
}

function _main {
  _demo_exactness
  _demo_invoice
  _demo_statistics
  _demo_rounding
  _demo_progress
  _demo_whole_numbers
  _demo_validation
  dybatpho::success "Math operations demo complete"
}

_main "$@"
