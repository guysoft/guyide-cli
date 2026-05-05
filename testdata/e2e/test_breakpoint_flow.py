"""End-to-end contract test: drive a real debugpy session through guyide.

This test is the executable spec for the debug-reach skill. If anything
here breaks, the skill will too. Run with::

    pytest -xvs testdata/e2e/

Set ``GUYIDE_E2E_NVGUY_LOCAL=/path/to/nvguy`` to use a local NvGuy clone
and skip the network fetch.
"""

from __future__ import annotations

import time
from typing import Any

import pytest


SCHEMA = "guyide/v1"


def _assert_envelope(doc: dict, *, level: str | None = None) -> None:
    assert doc.get("schema") == SCHEMA, f"missing/incorrect schema in {doc}"
    if level is not None:
        assert doc.get("level") == level, f"expected level={level} in {doc}"


@pytest.mark.timeout(420)
def test_breakpoint_flow(guyide_stack: Any) -> None:
    s = guyide_stack

    # 1. doctor reports ready (nvim is up; tmux is up; no debug session yet).
    doctor = s.cli("doctor")
    _assert_envelope(doctor)
    assert doctor.get("ready") is True, doctor
    assert doctor.get("failures", 1) == 0, doctor

    # 2. list-configs sees our launch.json.
    configs = s.cli("debug", "list-configs")
    _assert_envelope(configs, level="info")
    names = [c["name"] for c in configs.get("configs", [])]
    assert "sample-debugpy" in names, f"sample-debugpy not in {names}"

    # 3. set a breakpoint at sample.py:13 (the result = a + b line).
    sample = s.workdir / "sample.py"
    bp = s.cli("debug", "break", "set", "--file", str(sample), "--line", "14")
    _assert_envelope(bp, level="success")

    # 4. start the debug session.
    start = s.cli("debug", "start", "--config", "sample-debugpy")
    _assert_envelope(start, level="success")
    assert start.get("started") is True
    assert start.get("config") == "sample-debugpy"

    # 5. wait for the first stop (breakpoint reason).
    state = s.cli(
        "debug", "state", "--wait", "--reason", "breakpoint",
        "--timeout", "60s", "--frames", "--vars",
        timeout=90.0,
    )
    _assert_envelope(state)
    assert state.get("stopped") is True, state
    assert state.get("reason") == "breakpoint", state
    assert state.get("file", "").endswith("sample.py"), state
    assert state.get("line") == 14, state

    frames = state.get("frames") or []
    assert len(frames) >= 1, f"expected at least one frame, got {frames}"
    assert frames[0]["name"], frames[0]

    variables = state.get("variables") or []
    var_names = {v["name"] for v in variables}
    assert {"a", "b"}.issubset(var_names), (
        f"expected locals 'a' and 'b' at the breakpoint, got {var_names}"
    )

    # 6. continue: should hit the breakpoint again on the next loop iteration.
    cont = s.cli("debug", "continue")
    _assert_envelope(cont, level="success")
    # Give debugpy a beat to resume + re-stop.
    time.sleep(0.2)
    state2 = s.cli(
        "debug", "state", "--wait", "--reason", "breakpoint",
        "--timeout", "30s",
    )
    _assert_envelope(state2)
    assert state2.get("stopped") is True
    assert state2.get("reason") == "breakpoint"

    # 7. stop the session cleanly.
    stop = s.cli("debug", "stop")
    _assert_envelope(stop, level="success")

    # 8. final state shows session inactive.
    # Allow up to a couple of seconds for the terminate event to land.
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        st = s.cli("debug", "state")
        if not st.get("session_active"):
            break
        time.sleep(0.2)
    else:
        pytest.fail("session_active stayed true after debug stop")
