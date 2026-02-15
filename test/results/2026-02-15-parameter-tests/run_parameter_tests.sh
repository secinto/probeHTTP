#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="test/results/2026-02-15-parameter-tests"
INPUT_FILE="$BASE_DIR/input_urls.txt"

if [[ ! -f "$INPUT_FILE" ]]; then
  echo "Input file not found: $INPUT_FILE" >&2
  exit 1
fi

printf "test_name\texit_code\tduration_sec\tjson_lines\tstdout_lines\tstderr_lines\tflags\n" > "$BASE_DIR/run_summary.tsv"

run_test() {
  local test_name="$1"
  shift

  local json_file="$BASE_DIR/${test_name}.json"
  local stdout_file="$BASE_DIR/${test_name}.stdout.log"
  local stderr_file="$BASE_DIR/${test_name}.stderr.log"

  local start_ts
  local end_ts
  local duration
  local exit_code
  local json_lines=0
  local stdout_lines=0
  local stderr_lines=0

  start_ts=$(date +%s)
  set +e
  ./probeHTTP -i "$INPUT_FILE" -o "$json_file" -t 5 -c 40 "$@" > "$stdout_file" 2> "$stderr_file"
  exit_code=$?
  set -e
  end_ts=$(date +%s)
  duration=$((end_ts - start_ts))

  if [[ -f "$json_file" ]]; then
    json_lines=$(wc -l < "$json_file" | tr -d ' ')
  fi
  if [[ -f "$stdout_file" ]]; then
    stdout_lines=$(wc -l < "$stdout_file" | tr -d ' ')
  fi
  if [[ -f "$stderr_file" ]]; then
    stderr_lines=$(wc -l < "$stderr_file" | tr -d ' ')
  fi

  printf "%s\t%s\t%s\t%s\t%s\t%s\t%s\n" \
    "$test_name" "$exit_code" "$duration" "$json_lines" "$stdout_lines" "$stderr_lines" "$*" \
    >> "$BASE_DIR/run_summary.tsv"
}

run_test "01_default"
run_test "02_insecure" -k
run_test "03_all_schemes" -as
run_test "04_ignore_ports" -ip
run_test "05_custom_ports_443_8443" -p "443,8443"
run_test "06_no_redirects" -fr=false
run_test "07_disable_http3" --disable-http3
run_test "08_retries_2" --retries 2

echo "Completed parameter test run. Summary: $BASE_DIR/run_summary.tsv"
