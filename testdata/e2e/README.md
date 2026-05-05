# guyide e2e — debug-reach contract

This directory holds the executable spec for the debug-reach skill. If
this test passes, the skill's documented happy path works against a real
nvim+tmux+debugpy stack.

## What it does

`test_breakpoint_flow.py` drives the full guyide debug surface against a
freshly-installed NvGuy + vscodium.nvim + nvim-dap + debugpy:

1. `guyide doctor`            -> ready
2. `guyide debug list-configs` -> sees `sample-debugpy`
3. `guyide debug break set --file sample.py --line 13`
4. `guyide debug start --config sample-debugpy`
5. `guyide debug state --wait --reason breakpoint --vars --frames`
6. assert `a` and `b` are reported as locals at the stop
7. `guyide debug continue` -> hit again on the next loop iteration
8. `guyide debug stop`     -> `session_active` becomes `false`

Every step is asserted to carry the `"schema":"guyide/v1"` envelope.

## Running locally

```
python -m pip install pytest pytest-timeout debugpy
pytest -xvs testdata/e2e/
```

Optional env vars:

* `GUYIDE_E2E_NVGUY_LOCAL=/path/to/nvguy`  — use a local clone instead of
  fetching from GitHub.
* `GUYIDE_E2E_NVGUY_REF=<branch>`          — pin a different NvGuy ref
  (default: `main`).
* `GUYIDE_E2E_KEEP=1`                      — keep the ephemeral tmux
  server alive after the test for inspection.

## Known upstream blockers

The following nvim-launch.debug-rpc Lua issues will cause the test to
fail until they are patched in `vscodium.nvim`:

1. `list_configs()` calls `launch_json.load_configs` which does not
   exist (likely renamed). Step 2 will fail.
2. `step_over/into/out` blindly return `{success: true}` even when no
   dap session exists. Not exercised by this test, but worth tracking.

These are tracked in `BRANCH-NOTES.md` of the
`feature/cli-skill-bridge` branch.

## CI

`.github/workflows/e2e.yml` runs this on every push, every PR, and
nightly at 03:17 UTC. Diagnostics from a failing run are uploaded as the
`e2e-diagnostics` artifact.
