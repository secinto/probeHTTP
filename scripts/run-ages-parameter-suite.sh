#!/usr/bin/env bash
set -euo pipefail

BINARY="./probeHTTP"
INPUT_FILE="testdata/ages_parameter_suite_urls.txt"
BASE_RESULTS_DIR="test/results/parameter-suite"
OUT_DIR=""
TIMEOUT=5
CONCURRENCY=40
ASSERT_MODE=0
SELECTED_TESTS=""

ALL_TESTS=(
  "01_default"
  "02_insecure"
  "03_all_schemes"
  "04_ignore_ports"
  "05_custom_ports_443_8443"
  "06_no_redirects"
  "07_disable_http3"
  "08_retries_2"
  "09_all_schemes_ignore_ports"
  "10_all_schemes_ports_443_8443"
  "11_ignore_ports_plus_ports_443_8443"
  "12_all_schemes_ignore_ports_insecure"
  "13_insecure_disable_http3"
  "14_follow_redirects_max1"
  "15_same_host_only"
  "16_retries3_timeout3"
  "17_include_response_header"
  "18_include_response_full"
  "19_store_response"
)

usage() {
  cat <<'EOF'
Usage:
  scripts/run-ages-parameter-suite.sh [options]

Options:
  --binary <path>        Binary path (default: ./probeHTTP)
  --input <path>         Input URL file (default: testdata/ages_parameter_suite_urls.txt)
  --out-dir <path>       Output directory (default: test/results/parameter-suite/<timestamp>)
  --timeout <seconds>    Default timeout for requests (default: 5)
  --concurrency <n>      Concurrency (default: 40)
  --tests <csv>          Run only selected tests, e.g. "01_default,02_insecure"
  --assert               Enable structural assertions on run output
  -h, --help             Show this help

Outputs:
  - <out-dir>/<test_name>.json
  - <out-dir>/<test_name>.stdout.log
  - <out-dir>/<test_name>.stderr.log
  - <out-dir>/run_summary.tsv
  - <out-dir>/run_summary_detailed.tsv
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary)
      BINARY="$2"
      shift 2
      ;;
    --input)
      INPUT_FILE="$2"
      shift 2
      ;;
    --out-dir)
      OUT_DIR="$2"
      shift 2
      ;;
    --timeout)
      TIMEOUT="$2"
      shift 2
      ;;
    --concurrency)
      CONCURRENCY="$2"
      shift 2
      ;;
    --tests)
      SELECTED_TESTS="$2"
      shift 2
      ;;
    --assert)
      ASSERT_MODE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ ! -x "$BINARY" ]]; then
  echo "Binary not found or not executable: $BINARY" >&2
  exit 1
fi

if [[ ! -f "$INPUT_FILE" ]]; then
  echo "Input file not found: $INPUT_FILE" >&2
  exit 1
fi

if [[ -z "$OUT_DIR" ]]; then
  timestamp=$(date +"%Y%m%d-%H%M%S")
  OUT_DIR="$BASE_RESULTS_DIR/$timestamp"
fi

mkdir -p "$OUT_DIR"

SUMMARY_FILE="$OUT_DIR/run_summary.tsv"
DETAILED_FILE="$OUT_DIR/run_summary_detailed.tsv"

if [[ -n "$SELECTED_TESTS" ]]; then
  IFS=',' read -r -a TEST_LIST <<< "$SELECTED_TESTS"
else
  TEST_LIST=("${ALL_TESTS[@]}")
fi

printf "test_name\texit_code\tduration_sec\tjson_lines\tstdout_lines\tstderr_lines\tloaded_count\texpanded_count\ttotal\tsuccess\terrors\tflags\n" > "$SUMMARY_FILE"

run_test() {
  local test_name="$1"
  shift

  local json_file="$OUT_DIR/${test_name}.json"
  local stdout_file="$OUT_DIR/${test_name}.stdout.log"
  local stderr_file="$OUT_DIR/${test_name}.stderr.log"

  local start_ts
  local end_ts
  local duration
  local exit_code
  local json_lines=0
  local stdout_lines=0
  local stderr_lines=0

  local loaded_count=""
  local expanded_count=""
  local total=""
  local success=""
  local errors=""

  echo "Running $test_name ..."

  start_ts=$(date +%s)
  set +e
  "$BINARY" -i "$INPUT_FILE" -o "$json_file" -t "$TIMEOUT" -c "$CONCURRENCY" "$@" > "$stdout_file" 2> "$stderr_file"
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

  read -r loaded_count expanded_count total success errors < <(python3 - "$stderr_file" <<'PY'
import json
import sys

log_path = sys.argv[1]
loaded = ""
expanded = ""
total = ""
success = ""
errors = ""

with open(log_path, "r", encoding="utf-8") as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        try:
            event = json.loads(line)
        except Exception:
            continue

        msg = event.get("msg", "")
        if msg == "loaded URLs":
            loaded = str(event.get("count", ""))
        elif msg == "expanded URLs":
            expanded = str(event.get("count", ""))
        elif msg == "probing completed":
            total = str(event.get("total", ""))
            success = str(event.get("success", ""))
            errors = str(event.get("errors", ""))

print(loaded, expanded, total, success, errors)
PY
)

  printf "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n" \
    "$test_name" "$exit_code" "$duration" "$json_lines" "$stdout_lines" "$stderr_lines" "$loaded_count" "$expanded_count" "$total" "$success" "$errors" "$*" \
    >> "$SUMMARY_FILE"
}

run_named_test() {
  local name="$1"
  case "$name" in
    "01_default") run_test "$name" ;;
    "02_insecure") run_test "$name" -k ;;
    "03_all_schemes") run_test "$name" -as ;;
    "04_ignore_ports") run_test "$name" -ip ;;
    "05_custom_ports_443_8443") run_test "$name" -p "443,8443" ;;
    "06_no_redirects") run_test "$name" -fr=false ;;
    "07_disable_http3") run_test "$name" --disable-http3 ;;
    "08_retries_2") run_test "$name" --retries 2 ;;
    "09_all_schemes_ignore_ports") run_test "$name" -as -ip ;;
    "10_all_schemes_ports_443_8443") run_test "$name" -as -p "443,8443" ;;
    "11_ignore_ports_plus_ports_443_8443") run_test "$name" -ip -p "443,8443" ;;
    "12_all_schemes_ignore_ports_insecure") run_test "$name" -as -ip -k ;;
    "13_insecure_disable_http3") run_test "$name" -k --disable-http3 ;;
    "14_follow_redirects_max1") run_test "$name" -fr=true -maxr 1 ;;
    "15_same_host_only") run_test "$name" -sho ;;
    "16_retries3_timeout3") run_test "$name" --retries 3 -t 3 ;;
    "17_include_response_header") run_test "$name" -irh ;;
    "18_include_response_full") run_test "$name" -irr ;;
    "19_store_response")
      local response_dir="$OUT_DIR/19_store_response_responses"
      rm -rf "$response_dir"
      mkdir -p "$response_dir"
      run_test "$name" -sr -srd "$response_dir"
      ;;
    *)
      echo "Unknown test name: $name" >&2
      exit 1
      ;;
  esac
}

for test_name in "${TEST_LIST[@]}"; do
  run_named_test "$test_name"
done

python3 - "$SUMMARY_FILE" "$DETAILED_FILE" <<'PY'
import csv
import json
import os
import sys

summary_file = sys.argv[1]
detailed_file = sys.argv[2]
base_dir = os.path.dirname(summary_file)

rows = []
with open(summary_file, "r", encoding="utf-8") as f:
    reader = csv.DictReader(f, delimiter="\t")
    rows.extend(reader)

with open(detailed_file, "w", encoding="utf-8") as out:
    out.write(
        "test_name\tjson_lines\tstatus_gt0\tstatus_0\twith_error_field\tunique_inputs\t"
        "status_2xx\tstatus_3xx\tstatus_4xx\tstatus_5xx\n"
    )

    for row in rows:
        name = row["test_name"]
        json_file = os.path.join(base_dir, f"{name}.json")

        line_count = 0
        status_gt0 = 0
        status_0 = 0
        with_error_field = 0
        unique_inputs = set()
        status_2xx = 0
        status_3xx = 0
        status_4xx = 0
        status_5xx = 0

        if os.path.exists(json_file):
            with open(json_file, "r", encoding="utf-8") as jf:
                for line in jf:
                    line = line.strip()
                    if not line:
                        continue
                    line_count += 1
                    try:
                        obj = json.loads(line)
                    except Exception:
                        continue

                    inp = obj.get("input")
                    if inp:
                        unique_inputs.add(inp)

                    status = obj.get("status_code", 0)
                    if not isinstance(status, int):
                        status = 0

                    if status == 0:
                        status_0 += 1
                    else:
                        status_gt0 += 1
                        if 200 <= status < 300:
                            status_2xx += 1
                        elif 300 <= status < 400:
                            status_3xx += 1
                        elif 400 <= status < 500:
                            status_4xx += 1
                        elif 500 <= status < 600:
                            status_5xx += 1

                    if obj.get("error"):
                        with_error_field += 1

        out.write(
            f"{name}\t{line_count}\t{status_gt0}\t{status_0}\t{with_error_field}\t"
            f"{len(unique_inputs)}\t{status_2xx}\t{status_3xx}\t{status_4xx}\t{status_5xx}\n"
        )
PY

if [[ "$ASSERT_MODE" -eq 1 ]]; then
  input_count=$(grep -v '^[[:space:]]*$' "$INPUT_FILE" | wc -l | tr -d ' ')

  python3 - "$SUMMARY_FILE" "$input_count" <<'PY'
import csv
import sys

summary_file = sys.argv[1]
input_count = int(sys.argv[2])

expected_expanded = {
    "01_default": 83,
    "02_insecure": 83,
    "03_all_schemes": 166,
    "04_ignore_ports": 332,
    "05_custom_ports_443_8443": 166,
    "06_no_redirects": 83,
    "07_disable_http3": 83,
    "08_retries_2": 83,
    "09_all_schemes_ignore_ports": 664,
    "10_all_schemes_ports_443_8443": 332,
    "11_ignore_ports_plus_ports_443_8443": 166,
    "12_all_schemes_ignore_ports_insecure": 664,
    "13_insecure_disable_http3": 83,
    "14_follow_redirects_max1": 83,
    "15_same_host_only": 83,
    "16_retries3_timeout3": 83,
    "17_include_response_header": 83,
    "18_include_response_full": 83,
    "19_store_response": 83,
}

rows = {}
failures = []
with open(summary_file, "r", encoding="utf-8") as f:
    reader = csv.DictReader(f, delimiter="\t")
    for row in reader:
        rows[row["test_name"]] = row

for name, row in rows.items():
    exit_code = int(row["exit_code"])
    if exit_code != 0:
        failures.append(f"{name}: non-zero exit_code={exit_code}")

    loaded = row["loaded_count"]
    if loaded:
        if int(loaded) != input_count:
            failures.append(f"{name}: loaded_count={loaded}, expected={input_count}")

    expanded = row["expanded_count"]
    if name in expected_expanded and expanded:
        exp = expected_expanded[name]
        if int(expanded) != exp:
            failures.append(f"{name}: expanded_count={expanded}, expected={exp}")

# Relative assertions (network variability tolerant)
def as_int(test_name, field_name):
    value = rows.get(test_name, {}).get(field_name, "")
    return int(value) if value else None

default_success = as_int("01_default", "success")
insecure_success = as_int("02_insecure", "success")
if default_success is not None and insecure_success is not None:
    if insecure_success < default_success:
        failures.append(
            f"02_insecure success ({insecure_success}) should be >= 01_default ({default_success})"
        )

broad_success = as_int("09_all_schemes_ignore_ports", "success")
broad_insecure_success = as_int("12_all_schemes_ignore_ports_insecure", "success")
if broad_success is not None and broad_insecure_success is not None:
    if broad_insecure_success < broad_success:
        failures.append(
            f"12_all_schemes_ignore_ports_insecure success ({broad_insecure_success}) should be >= 09_all_schemes_ignore_ports ({broad_success})"
        )

if failures:
    print("ASSERTIONS FAILED:")
    for f in failures:
        print(f"- {f}")
    sys.exit(1)

print("Assertions passed.")
PY
fi

echo "Parameter suite completed."
echo "Summary:  $SUMMARY_FILE"
echo "Details:  $DETAILED_FILE"
