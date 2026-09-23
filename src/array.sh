#!/usr/bin/env bash
# @file array.sh
# @brief Utilities for working with array
# @description
#   This module contains helpers for printing, reversing, deduplicating,
#   compacting, filtering, mapping, rejecting, finding values, checking
#   membership, checking every/some values, finding positions, and joining Bash
#   arrays by name. It also sorts and slices them, and treats them as sets for
#   union, intersection, and difference.
#
#   Every helper takes an array by name and changes it in place, with a final
#   `--` to print the result as well.
# @see
#   - `example/array_ops.sh`
: "${DYBATPHO_DIR:?DYBATPHO_DIR must be set. Please source dybatpho/init.sh before other scripts from dybatpho.}"

#######################################
# @description Print each element of an array on its own line.
# @arg $1 string Name of array
# @stdout Print array with each element separated by newline
#######################################
function dybatpho::array_print {
  local -n input_arr="$1"
  printf '%s\n' "${input_arr[@]}"
}

#######################################
# @description Reverse an array in place.
# @arg $1 string Name of array
# @arg $2 string Set `--` to print to stdout
# @stdout Print the reversed array if $2 is `--`
#######################################
function dybatpho::array_reverse {
  local -n input_arr="$1"
  local result_arr=()
  [ "${#input_arr[@]}" -eq 0 ] && return
  local -a indices=("${!input_arr[@]}")

  for ((i = ${#indices[@]} - 1; i >= 0; i--)); do
    # shellcheck disable=SC2190 # result_arr is indexed; the nameref misleads ShellCheck
    result_arr+=("${input_arr[${indices[${i}]}]}")
  done

  input_arr=("${result_arr[@]}")
  if [[ "${2-""}" == "--" ]]; then
    dybatpho::array_print "$1"
  fi
}

#######################################
# @description Remove duplicate elements from an array in place.
# @arg $1 string Name of array
# @arg $2 string Set `--` to print to stdout
# @stdout Print the deduplicated array if $2 is `--`
#######################################
function dybatpho::array_unique {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  declare -A result_arr

  for i in "${input_arr[@]}"; do
    [[ ${i} ]] && IFS=" " result_arr["${i:- }"]=1
  done

  input_arr=("${!result_arr[@]}")
  if [[ "${2-""}" == "--" ]]; then
    dybatpho::array_print "$1"
  fi
}

#######################################
# @description Return success when an array contains the given element.
# @arg $1 string Name of array
# @arg $2 string Element to search for
# @exitcode 0 The element exists in the array
# @exitcode 1 The element does not exist in the array
#######################################
function dybatpho::array_contains {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  local needle="${2-}"
  local i
  for i in "${!input_arr[@]}"; do
    [[ "${input_arr[${i}]}" == "${needle}" ]] && return 0
  done
  return 1
}

#######################################
# @description Print the first index of an array element that matches exactly.
# @arg $1 string Name of array
# @arg $2 string Element to search for
# @stdout First matching index
# @exitcode 0 A matching element is found
# @exitcode 1 No matching element is found
#######################################
function dybatpho::array_index_of {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  local needle="${2-}"
  local i
  for i in "${!input_arr[@]}"; do
    if [[ "${input_arr[${i}]}" == "${needle}" ]]; then
      printf '%s\n' "${i}"
      return 0
    fi
  done
  return 1
}

#######################################
# @description Remove empty-string elements from an array in place.
# @arg $1 string Name of array
# @arg $2 string Set `--` to print to stdout
# @stdout Print the compacted array if $2 is `--`
#######################################
function dybatpho::array_compact {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  local compacted_arr=()
  local i
  for i in "${!input_arr[@]}"; do
    [[ -n "${input_arr[${i}]}" ]] && compacted_arr+=("${input_arr[${i}]}")
  done
  input_arr=("${compacted_arr[@]}")
  if [[ "${2-""}" == "--" ]]; then
    dybatpho::array_print "$1"
  fi
}

#######################################
# @description Keep only array elements accepted by a predicate function.
# @arg $1 string Name of array
# @arg $2 string Predicate function name, called with each element
# @arg $3 string Set `--` to print to stdout
# @stdout Print the filtered array if $3 is `--`
#######################################
function dybatpho::array_filter {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  local predicate="${2-}"
  local filtered_arr=()
  local i
  dybatpho::is function "${predicate}" || dybatpho::die "Invalid predicate function: ${predicate}"
  for i in "${!input_arr[@]}"; do
    if "${predicate}" "${input_arr[${i}]}"; then
      filtered_arr+=("${input_arr[${i}]}")
    fi
  done
  input_arr=("${filtered_arr[@]}")
  if [[ "${3-""}" == "--" ]]; then
    dybatpho::array_print "$1"
  fi
}

#######################################
# @description Transform each array element with a mapper function.
# @arg $1 string Name of array
# @arg $2 string Mapper function name, called with each element
# @arg $3 string Set `--` to print to stdout
# @stdout Print the mapped array if $3 is `--`
#######################################
function dybatpho::array_map {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  local mapper="${2-}"
  local mapped_arr=()
  local mapped_value i status
  dybatpho::is function "${mapper}" || dybatpho::die "Invalid mapper function: ${mapper}"
  for i in "${!input_arr[@]}"; do
    mapped_value=$("${mapper}" "${input_arr[${i}]}")
    status=$?
    ((status == 0)) || return "${status}"
    mapped_arr+=("${mapped_value}")
  done
  input_arr=("${mapped_arr[@]}")
  if [[ "${3-""}" == "--" ]]; then
    dybatpho::array_print "$1"
  fi
}

#######################################
# @description Print the first array element accepted by a predicate function.
# @arg $1 string Name of array
# @arg $2 string Predicate function name, called with each element
# @stdout First matching array element
# @exitcode 0 A matching element is found
# @exitcode 1 No matching element is found
#######################################
function dybatpho::array_find {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  local predicate="${2-}"
  local i
  dybatpho::is function "${predicate}" || dybatpho::die "Invalid predicate function: ${predicate}"
  for i in "${!input_arr[@]}"; do
    if "${predicate}" "${input_arr[${i}]}"; then
      printf '%s\n' "${input_arr[${i}]}"
      return 0
    fi
  done
  return 1
}

#######################################
# @description Return success when every array element is accepted by a predicate function.
# @arg $1 string Name of array
# @arg $2 string Predicate function name, called with each element
# @exitcode 0 Every element matches, or the array is empty
# @exitcode 1 At least one element does not match
#######################################
function dybatpho::array_every {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  local predicate="${2-}"
  local i
  dybatpho::is function "${predicate}" || dybatpho::die "Invalid predicate function: ${predicate}"
  for i in "${!input_arr[@]}"; do
    "${predicate}" "${input_arr[${i}]}" || return 1
  done
  return 0
}

#######################################
# @description Return success when at least one array element is accepted by a predicate function.
# @arg $1 string Name of array
# @arg $2 string Predicate function name, called with each element
# @exitcode 0 At least one element matches
# @exitcode 1 No elements match
#######################################
function dybatpho::array_some {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  local predicate="${2-}"
  local i
  dybatpho::is function "${predicate}" || dybatpho::die "Invalid predicate function: ${predicate}"
  for i in "${!input_arr[@]}"; do
    if "${predicate}" "${input_arr[${i}]}"; then
      return 0
    fi
  done
  return 1
}

#######################################
# @description Keep only array elements rejected by a predicate function.
# @arg $1 string Name of array
# @arg $2 string Predicate function name, called with each element
# @arg $3 string Set `--` to print to stdout
# @stdout Print the rejected array if $3 is `--`
#######################################
function dybatpho::array_reject {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  local predicate="${2-}"
  local rejected_arr=()
  local i
  dybatpho::is function "${predicate}" || dybatpho::die "Invalid predicate function: ${predicate}"
  for i in "${!input_arr[@]}"; do
    if ! "${predicate}" "${input_arr[${i}]}"; then
      rejected_arr+=("${input_arr[${i}]}")
    fi
  done
  input_arr=("${rejected_arr[@]}")
  if [[ "${3-""}" == "--" ]]; then
    dybatpho::array_print "$1"
  fi
}

#######################################
# @description Print the first element of an array.
# @arg $1 string Name of array
# @stdout First array element
# @exitcode 0 The array contains at least one element
# @exitcode 1 The array is empty
#######################################
function dybatpho::array_first {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  local -a indices=("${!input_arr[@]}")
  ((${#indices[@]} > 0)) || return 1
  printf '%s\n' "${input_arr[${indices[0]}]}"
}

#######################################
# @description Print the last element of an array.
# @arg $1 string Name of array
# @stdout Last array element
# @exitcode 0 The array contains at least one element
# @exitcode 1 The array is empty
#######################################
function dybatpho::array_last {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  local -a indices=("${!input_arr[@]}")
  local last_index
  ((${#indices[@]} > 0)) || return 1
  last_index=$((${#indices[@]} - 1))
  printf '%s\n' "${input_arr[${indices[${last_index}]}]}"
}

#######################################
# @description Join array elements with a separator into one string.
# @arg $1 string Name of array
# @arg $2 string Separator
# @stdout Print outputted string
#######################################
function dybatpho::array_join {
  # shellcheck disable=SC2178
  local -n input_arr="$1"
  local separator="$2"
  local i

  if [[ ${#input_arr[@]} -eq 0 ]]; then
    return
  fi
  printf -- "%s" "${input_arr[0]}"
  for ((i = 1; i < ${#input_arr[@]}; i++)); do
    printf -- "%s%s" "${separator}" "${input_arr[${i}]}"
  done
}

#######################################
# @description Copy the values of one array into another.
#   Bash 4.3 treats `"${empty[@]}"` as unset under `nounset`, so every copy in
#   this module goes through the length check here rather than repeating it.
# @arg $1 string Name of the array to fill
# @arg $2 string Name of the array to read
# @set The named array
#######################################
function __dybatpho_array_copy {
  local -n __copy_out="$1"
  # shellcheck disable=SC2178
  local -n __copy_in="$2"
  __copy_out=()
  ((${#__copy_in[@]} == 0)) || __copy_out=("${__copy_in[@]}")
}

#######################################
# @description Build a lookup of the values an array holds.
# @arg $1 string Name of the associative array to fill
# @arg $2 string Name of the array to read
# @set The named associative array, one key per distinct value
#######################################
function __dybatpho_array_index {
  local -n __index_out="$1"
  # shellcheck disable=SC2178
  local -n __index_in="$2"
  __index_out=()
  local __index_value
  for __index_value in ${__index_in[@]+"${__index_in[@]}"}; do
    __index_out["${__index_value}"]=1
  done
}

#######################################
# @description Return success when one value must sort after another.
# @arg $1 string Left value
# @arg $2 string Right value
# @arg $3 bool Compare as numbers rather than as text
# @arg $4 bool Reverse the order
# @exitcode 0 The left value belongs after the right one
# @exitcode 1 It does not
#######################################
function __dybatpho_array_sorts_after {
  local left="$1" right="$2" numeric="$3" reverse="$4"
  local order
  if [[ "${numeric}" == true ]]; then
    if ((left > right)); then
      order=1
    elif ((left < right)); then
      order=-1
    else
      order=0
    fi
  else
    if [[ "${left}" > "${right}" ]]; then
      order=1
    elif [[ "${left}" < "${right}" ]]; then
      order=-1
    else
      order=0
    fi
  fi
  if [[ "${reverse}" == true ]]; then
    ((order < 0))
  else
    ((order > 0))
  fi
}

#######################################
# @description Sort an array in place.
#   Text is ordered by the current locale's collation, the same rule `sort`
#   follows, so a script that needs one fixed order everywhere sets `LC_ALL` as
#   it would for `sort`.
#
#   `--numeric` compares values as numbers, which is the reason a shell script
#   wants a sort at all: as text, `10` comes before `9`. It takes integers,
#   negative ones included, and stops the script on anything else rather than
#   quietly ordering it as text.
#
#   The sort is an insertion sort rather than a pipe through `sort(1)`: it keeps
#   an element containing a newline intact, needs no external command, and is
#   quick at the sizes a shell array actually reaches.
# @example
#   releases=(1.10 1.9 2.0)
#   dybatpho::array_sort releases --
#   # 1.10
#   # 1.9
#   # 2.0
#
# @example
#   sizes=(10 9 100 -3)
#   dybatpho::array_sort sizes --numeric --          # -3 9 10 100
#   dybatpho::array_sort sizes --numeric --reverse
#
# @arg $1 string Name of array
# @arg $@ string Any of `--numeric`/`-n`, `--reverse`/`-r`, and `--` to print
# @stdout Print the sorted array if `--` is given
# @exitcode 1 Stop the script on an unknown option, or on a value that is not an integer under `--numeric`
# @see
#   - `dybatpho::semver_sort`
#######################################
function dybatpho::array_sort {
  # The locals carry a distinctive prefix because a nameref resolves in the
  # caller's scope: a plainly named local here would shadow a caller's array of
  # the same name, and this function would then sort its own empty copy.
  local __sort_numeric=false __sort_reverse=false __sort_print=false
  local __sort_option
  for __sort_option in "${@:2}"; do
    case "${__sort_option}" in
      -n | --numeric) __sort_numeric=true ;;
      -r | --reverse) __sort_reverse=true ;;
      --) __sort_print=true ;;
      *) dybatpho::die "dybatpho::array_sort: Unknown option '${__sort_option}'" ;;
    esac
  done

  local -a __sort_values=()
  __dybatpho_array_copy __sort_values "$1"
  local __sort_count="${#__sort_values[@]}"
  local __sort_value
  if [[ "${__sort_numeric}" == true ]]; then
    for __sort_value in ${__sort_values[@]+"${__sort_values[@]}"}; do
      [[ "${__sort_value}" =~ ^-?[0-9]+$ ]] \
        || dybatpho::die "dybatpho::array_sort: '${__sort_value}' is not an integer; drop --numeric, or compare with the math module"
    done
  fi

  local __sort_index __sort_position __sort_current
  for ((__sort_index = 1; __sort_index < __sort_count; __sort_index++)); do
    __sort_current="${__sort_values[${__sort_index}]}"
    __sort_position=$((__sort_index - 1))
    while ((__sort_position >= 0)) \
      && __dybatpho_array_sorts_after "${__sort_values[${__sort_position}]}" "${__sort_current}" "${__sort_numeric}" "${__sort_reverse}"; do
      __sort_values[__sort_position + 1]="${__sort_values[${__sort_position}]}"
      __sort_position=$((__sort_position - 1))
    done
    __sort_values[__sort_position + 1]="${__sort_current}"
  done

  __dybatpho_array_copy "$1" __sort_values
  if [[ "${__sort_print}" == true ]]; then
    dybatpho::array_print "$1"
  fi
}

#######################################
# @description Keep a run of an array in place and drop the rest.
#   A negative start counts back from the end, so `-2` takes the last two
#   elements without the caller working out the length first. A start past
#   either end leaves an empty array rather than failing: asking for elements
#   that are not there is a shape the data can have, not a mistake in the call.
# @example
#   items=(a b c d e)
#   dybatpho::array_slice items 1 3 --   # b c d
#   dybatpho::array_slice items -2 --    # the last two
#
# @arg $1 string Name of array
# @arg $2 number Index to start at, negative to count back from the end
# @arg $3 number Optional count, defaulting to everything from the start on
# @arg $4 string Set `--` to print to stdout
# @stdout Print the sliced array if `--` is given
# @exitcode 1 Stop the script when the start or the count is not a whole number
#######################################
function dybatpho::array_slice {
  local __slice_start="${2-}"
  [[ "${__slice_start}" =~ ^-?[0-9]+$ ]] \
    || dybatpho::die "dybatpho::array_slice: '${__slice_start}' is not a whole number"

  local -a __slice_values=()
  __dybatpho_array_copy __slice_values "$1"
  local __slice_count="${#__slice_values[@]}"

  # The count is optional, so the third argument is either it or the `--` that
  # would otherwise be fourth.
  local __slice_length="${__slice_count}" __slice_print="${4-}"
  if [[ "${3-}" == "--" ]]; then
    __slice_print="--"
  elif [[ -n "${3-}" ]]; then
    [[ "${3}" =~ ^[0-9]+$ ]] \
      || dybatpho::die "dybatpho::array_slice: '${3}' is not a count"
    __slice_length="${3}"
  fi

  if ((__slice_start < 0)); then
    __slice_start=$((__slice_count + __slice_start))
    ((__slice_start >= 0)) || __slice_start=0
  fi

  local -a __slice_result=()
  local __slice_index
  for ((__slice_index = __slice_start; __slice_index < __slice_count && __slice_index < __slice_start + __slice_length; __slice_index++)); do
    __slice_result+=("${__slice_values[${__slice_index}]}")
  done

  __dybatpho_array_copy "$1" __slice_result
  if [[ "${__slice_print}" == "--" ]]; then
    dybatpho::array_print "$1"
  fi
}

#######################################
# @description Replace an array with the union of it and another, in place.
#   The result is a set: every value appears once, in the order it was first
#   seen, the first array's values ahead of the second's. A set operation that
#   kept duplicates would not be one, so `dybatpho::array_unique` afterwards has
#   nothing left to do.
# @example
#   allowed=(read write read)
#   extra=(write admin)
#   dybatpho::array_union allowed extra --   # read write admin
#
# @arg $1 string Name of the array to replace
# @arg $2 string Name of the array to merge in
# @arg $3 string Set `--` to print to stdout
# @stdout Print the union if $3 is `--`
#######################################
function dybatpho::array_union {
  local -a __set_values=() __set_addition=()
  __dybatpho_array_copy __set_values "$1"
  __dybatpho_array_copy __set_addition "$2"
  ((${#__set_addition[@]} == 0)) || __set_values+=("${__set_addition[@]}")

  local -A __set_seen=()
  local -a __set_result=()
  local __set_value
  for __set_value in ${__set_values[@]+"${__set_values[@]}"}; do
    if [[ ! -v "__set_seen[${__set_value}]" ]]; then
      __set_seen["${__set_value}"]=1
      __set_result+=("${__set_value}")
    fi
  done

  __dybatpho_array_copy "$1" __set_result
  if [[ "${3-}" == "--" ]]; then
    dybatpho::array_print "$1"
  fi
}

#######################################
# @description Keep only the values an array shares with another, in place.
#   The result is a set, in the order the first array had them.
# @example
#   requested=(read write admin)
#   granted=(write read)
#   dybatpho::array_intersect requested granted --   # read write
#
# @arg $1 string Name of the array to replace
# @arg $2 string Name of the array to intersect with
# @arg $3 string Set `--` to print to stdout
# @stdout Print the intersection if $3 is `--`
#######################################
function dybatpho::array_intersect {
  local -A __set_other=()
  __dybatpho_array_index __set_other "$2"
  local -a __set_values=()
  __dybatpho_array_copy __set_values "$1"

  local -A __set_seen=()
  local -a __set_result=()
  local __set_value
  for __set_value in ${__set_values[@]+"${__set_values[@]}"}; do
    if [[ -v "__set_other[${__set_value}]" && ! -v "__set_seen[${__set_value}]" ]]; then
      __set_seen["${__set_value}"]=1
      __set_result+=("${__set_value}")
    fi
  done

  __dybatpho_array_copy "$1" __set_result
  if [[ "${3-}" == "--" ]]; then
    dybatpho::array_print "$1"
  fi
}

#######################################
# @description Drop the values an array shares with another, in place.
#   The result is a set, in the order the first array had them. The operation is
#   one-sided: values only the second array holds are not added.
# @example
#   wanted=(read write admin)
#   granted=(write)
#   dybatpho::array_difference wanted granted --   # read admin
#
# @arg $1 string Name of the array to replace
# @arg $2 string Name of the array to subtract
# @arg $3 string Set `--` to print to stdout
# @stdout Print the difference if $3 is `--`
#######################################
function dybatpho::array_difference {
  local -A __set_other=()
  __dybatpho_array_index __set_other "$2"
  local -a __set_values=()
  __dybatpho_array_copy __set_values "$1"

  local -A __set_seen=()
  local -a __set_result=()
  local __set_value
  for __set_value in ${__set_values[@]+"${__set_values[@]}"}; do
    if [[ ! -v "__set_other[${__set_value}]" && ! -v "__set_seen[${__set_value}]" ]]; then
      __set_seen["${__set_value}"]=1
      __set_result+=("${__set_value}")
    fi
  done

  __dybatpho_array_copy "$1" __set_result
  if [[ "${3-}" == "--" ]]; then
    dybatpho::array_print "$1"
  fi
}
