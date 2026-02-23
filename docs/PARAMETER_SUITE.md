# AGES Parameter Regression Suite

This suite replays the full probeHTTP parameter matrix against the AGES target list used for manual validation.

## Input fixture

- `testdata/ages_parameter_suite_urls.txt` (83 URLs)

## Run

```bash
# Run full suite
./scripts/run-ages-parameter-suite.sh

# Run full suite + assertions
./scripts/run-ages-parameter-suite.sh --assert

# Run subset
./scripts/run-ages-parameter-suite.sh --tests "01_default,02_insecure,13_insecure_disable_http3"

# Custom output directory
./scripts/run-ages-parameter-suite.sh --out-dir test/results/parameter-suite/latest
```

Or via Makefile:

```bash
make parameter-suite
```

## Output

Each run creates a timestamped directory under `test/results/parameter-suite/` with:

- `<test_name>.json`
- `<test_name>.stdout.log`
- `<test_name>.stderr.log`
- `run_summary.tsv`
- `run_summary_detailed.tsv`

## Test matrix

### Single-flag coverage

1. `01_default`
2. `02_insecure` (`-k`)
3. `03_all_schemes` (`-as`)
4. `04_ignore_ports` (`-ip`)
5. `05_custom_ports_443_8443` (`-p 443,8443`)
6. `06_no_redirects` (`-fr=false`)
7. `07_disable_http3` (`--disable-http3`)
8. `08_retries_2` (`--retries 2`)

### Combination coverage

9. `09_all_schemes_ignore_ports` (`-as -ip`)
10. `10_all_schemes_ports_443_8443` (`-as -p 443,8443`)
11. `11_ignore_ports_plus_ports_443_8443` (`-ip -p 443,8443`)
12. `12_all_schemes_ignore_ports_insecure` (`-as -ip -k`)
13. `13_insecure_disable_http3` (`-k --disable-http3`)
14. `14_follow_redirects_max1` (`-fr=true -maxr 1`)
15. `15_same_host_only` (`-sho`)
16. `16_retries3_timeout3` (`--retries 3 -t 3`)
17. `17_include_response_header` (`-irh`)
18. `18_include_response_full` (`-irr`)
19. `19_store_response` (`-sr -srd <dir>`)

## Assertions (`--assert`)

The suite validates:

- all tests exit with code `0`
- `loaded_count` equals fixture size
- deterministic `expanded_count` per test matrix
- `02_insecure.success >= 01_default.success`
- `12_all_schemes_ignore_ports_insecure.success >= 09_all_schemes_ignore_ports.success`

This keeps checks stable while still allowing natural network variability.
