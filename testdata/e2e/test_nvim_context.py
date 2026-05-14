"""E2E test: nvim exec/eval responses always include context fields.

Verifies that every nvim exec and eval response includes socket, buffer,
and cwd so that AI agents can detect when they are talking to the wrong
nvim instance. This test exists to prevent regressions — the context
fields must survive refactors.

Run with::

    pytest -xvs testdata/e2e/test_nvim_context.py
"""

from __future__ import annotations

from typing import Any

import pytest


SCHEMA = "guyide/v1"
CONTEXT_KEYS = {"socket", "buffer", "cwd"}


def _assert_context_present(doc: dict, label: str) -> None:
    """Assert that socket, buffer, and cwd are present and non-empty strings."""
    for key in CONTEXT_KEYS:
        val = doc.get(key)
        assert isinstance(val, str) and val, (
            f"{label}: expected non-empty string for '{key}', got {val!r}.\n"
            f"Full response: {doc}"
        )


@pytest.mark.timeout(120)
class TestNvimContextFields:
    """nvim exec and eval must always return socket, buffer, cwd."""

    def test_exec_returns_context(self, guyide_stack: Any) -> None:
        """guyide nvim exec should include socket, buffer, cwd in JSON."""
        result = guyide_stack.cli("nvim", "exec", "echo 'hello'")
        assert result.get("schema") == SCHEMA
        assert result.get("level") == "success"
        _assert_context_present(result, "nvim exec")

    def test_eval_returns_context(self, guyide_stack: Any) -> None:
        """guyide nvim eval should include socket, buffer, cwd in JSON."""
        result = guyide_stack.cli("nvim", "eval", "1+1")
        assert result.get("schema") == SCHEMA
        assert result.get("level") == "info"
        assert result.get("value") == 2
        _assert_context_present(result, "nvim eval")

    def test_exec_context_matches_actual_buffer(self, guyide_stack: Any) -> None:
        """After opening a file via exec, buffer field should reflect it."""
        sample = str(guyide_stack.workdir / "sample.py")
        guyide_stack.cli("nvim", "exec", f"edit {sample}")
        result = guyide_stack.cli("nvim", "exec", "echo 'check'")
        assert result["buffer"].endswith("sample.py"), (
            f"Expected buffer to end with sample.py, got {result['buffer']}"
        )

    def test_exec_context_cwd_matches_workdir(self, guyide_stack: Any) -> None:
        """cwd should match the nvim working directory."""
        result = guyide_stack.cli("nvim", "eval", "getcwd()")
        cwd_from_eval = result["value"]
        # The context cwd should match what getcwd() returns directly.
        assert result["cwd"] == cwd_from_eval, (
            f"Context cwd {result['cwd']!r} != eval getcwd() {cwd_from_eval!r}"
        )

    def test_exec_socket_is_valid_path(self, guyide_stack: Any) -> None:
        """socket field should point to the actual socket file."""
        result = guyide_stack.cli("nvim", "exec", "echo 1")
        assert result["socket"] == str(guyide_stack.nvim_sock), (
            f"Socket mismatch: response has {result['socket']!r}, "
            f"expected {str(guyide_stack.nvim_sock)!r}"
        )
