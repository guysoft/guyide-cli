"""Shared pytest fixtures for the guyide end-to-end debug flow.

These fixtures stand up a real, isolated stack:

* ephemeral tmux server (``-L guyide-e2e-<pid>``)
* fresh nvim with NvGuy as $XDG_CONFIG_HOME/nvim
* lazy.nvim sync waited on via guyide tmux watch (closing the loop:
  the e2e is itself a guyide consumer)
* socket discovered through guyide env --json

Tests should depend on the ``guyide_stack`` fixture for the full setup.
"""

from __future__ import annotations

import json
import os
import shutil
import socket
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import pytest


REPO_ROOT = Path(__file__).resolve().parents[2]
E2E_DIR = Path(__file__).resolve().parent
NVGUY_REPO = os.environ.get("GUYIDE_E2E_NVGUY_REPO", "https://github.com/guysoft/NvGuy.git")
NVGUY_REF = os.environ.get("GUYIDE_E2E_NVGUY_REF", "main")


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _run(cmd: list[str], **kwargs: Any) -> subprocess.CompletedProcess:
    """Run a command, raising with stderr captured on failure."""
    kwargs.setdefault("check", True)
    kwargs.setdefault("text", True)
    kwargs.setdefault("capture_output", True)
    return subprocess.run(cmd, **kwargs)


def _tmux(socket_name: str, *args: str, check: bool = True) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["tmux", "-L", socket_name, *args],
        text=True,
        capture_output=True,
        check=check,
    )


def _wait_for(predicate, timeout: float, interval: float = 0.2, what: str = "condition") -> None:
    deadline = time.monotonic() + timeout
    last_err: Exception | None = None
    while time.monotonic() < deadline:
        try:
            if predicate():
                return
        except Exception as exc:  # noqa: BLE001
            last_err = exc
        time.sleep(interval)
    raise TimeoutError(f"timed out after {timeout}s waiting for {what} (last_err={last_err})")


def _detect_debugpy_python() -> str | None:
    """Return a python interpreter that has debugpy importable, or None.

    Probes (in order): the running test interpreter, then any python on PATH
    via shutil.which. Caller is expected to set NVGUY_DEBUGPY_PYTHON, which
    NvGuy's debug.lua will hand to dap-python's setup().
    """
    import sys
    candidates = [sys.executable]
    for name in ("python3", "python"):
        path = shutil.which(name)
        if path and path not in candidates:
            candidates.append(path)
    for py in candidates:
        if not py:
            continue
        try:
            r = subprocess.run(
                [py, "-c", "import debugpy"],
                capture_output=True, timeout=5,
            )
            if r.returncode == 0:
                return py
        except (OSError, subprocess.TimeoutExpired):
            continue
    return None


def _find_nvim_socket(runtime_dir: Path) -> Path | None:
    """Return the most recently-modified nvim socket in runtime_dir, if any."""
    candidates = sorted(runtime_dir.glob("nvim.*.0"), key=lambda p: p.stat().st_mtime, reverse=True)
    for p in candidates:
        # Probe: only count it if we can connect.
        try:
            with socket.socket(socket.AF_UNIX) as s:
                s.settimeout(0.5)
                s.connect(str(p))
                return p
        except OSError:
            continue
    return None


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@dataclass
class Stack:
    guyide: Path
    tmux_socket: str
    session: str
    nvim_pane: str
    nvim_sock: Path
    workdir: Path
    xdg_config_home: Path
    xdg_data_home: Path
    runtime_dir: Path

    def cli(self, *args: str, json_out: bool = True, check: bool = True, timeout: float = 30.0) -> dict | str:
        """Invoke the guyide binary with --socket pointed at the e2e nvim."""
        cmd = [str(self.guyide), "--socket", str(self.nvim_sock)]
        if json_out:
            cmd.append("--json")
        cmd.extend(args)
        proc = subprocess.run(cmd, text=True, capture_output=True, timeout=timeout)
        if check and proc.returncode != 0:
            raise AssertionError(
                f"guyide {' '.join(args)} exited {proc.returncode}\nSTDOUT:\n{proc.stdout}\nSTDERR:\n{proc.stderr}"
            )
        if json_out:
            try:
                return json.loads(proc.stdout)
            except json.JSONDecodeError as exc:
                raise AssertionError(
                    f"guyide {' '.join(args)} did not produce JSON: {exc}\nSTDOUT:\n{proc.stdout}\nSTDERR:\n{proc.stderr}"
                ) from exc
        return proc.stdout


@pytest.fixture(scope="session")
def guyide_bin(tmp_path_factory: pytest.TempPathFactory) -> Path:
    """Build the guyide binary once per session under a temp dir."""
    out_dir = tmp_path_factory.mktemp("guyide-bin")
    binary = out_dir / "guyide"
    _run(
        ["go", "build", "-o", str(binary), "./cmd/guyide"],
        cwd=REPO_ROOT,
    )
    assert binary.exists(), "guyide binary was not produced"
    return binary


@pytest.fixture(scope="session")
def nvguy_clone(tmp_path_factory: pytest.TempPathFactory) -> Path:
    """Clone the NvGuy config once per session (or reuse a local override).

    Set GUYIDE_E2E_NVGUY_LOCAL=/path/to/nvguy to avoid the network clone.
    """
    local = os.environ.get("GUYIDE_E2E_NVGUY_LOCAL")
    if local:
        return Path(local).resolve()
    target = tmp_path_factory.mktemp("nvguy") / "nvguy"
    _run(["git", "clone", "--depth", "1", "--branch", NVGUY_REF, NVGUY_REPO, str(target)])
    return target


@pytest.fixture(scope="session")
def nvguy_prewarm(
    tmp_path_factory: pytest.TempPathFactory,
    nvguy_clone: Path,
) -> dict[str, Path]:
    """Pre-install lazy plugins and generate the base46 cache once per session.

    NvGuy's init.lua calls ``dofile(base46_cache .. "defaults")`` at startup,
    which fails on a cold install before NvChad has had a chance to run
    ``base46.load_all_highlights()``. Doing the install + cache priming in
    a headless run before any interactive session avoids racing the lazy
    install UI and the press-ENTER prompt.

    Returns a dict with the paths to the warmed-up XDG dirs. Per-test
    fixtures snapshot these into their own dirs to keep tests isolated.
    """
    warm_root = tmp_path_factory.mktemp("nvguy-warm")
    config = warm_root / "xdg-config"
    data = warm_root / "xdg-data"
    state = warm_root / "xdg-state"
    cache = warm_root / "xdg-cache"
    runtime = warm_root / "runtime"
    home = warm_root / "home"
    for p in (config, data, state, cache, runtime, home):
        p.mkdir(parents=True)
    runtime.chmod(0o700)

    shutil.copytree(
        nvguy_clone, config / "nvim",
        symlinks=True, ignore=shutil.ignore_patterns(".git"),
    )

    env = {
        **os.environ,
        "HOME": str(home),
        "XDG_CONFIG_HOME": str(config),
        "XDG_DATA_HOME": str(data),
        "XDG_STATE_HOME": str(state),
        "XDG_CACHE_HOME": str(cache),
        "XDG_RUNTIME_DIR": str(runtime),
        # Headless mode: avoid TUI surprises.
        "NVIM_LISTEN_ADDRESS": "",
        "TMUX": "",
    }

    # Optional: tell NvGuy's debug.lua spec to point lazy at a local
    # vscodium.nvim checkout instead of cloning from GitHub. Used by the
    # e2e suite to exercise unpushed nvim-launch.debug-rpc changes.
    vscodium_local = os.environ.get("GUYIDE_E2E_VSCODIUM_LOCAL")
    if vscodium_local:
        env["NVGUY_VSCODIUM_LOCAL"] = str(Path(vscodium_local).resolve())
        print(f"[nvguy_prewarm] NVGUY_VSCODIUM_LOCAL={env['NVGUY_VSCODIUM_LOCAL']}", flush=True)

    # Step 1: install plugins. NvGuy's init.lua dofile()s the base46 cache
    # at startup, which won't exist on a cold install. Build a bootstrap
    # init by stripping those two dofile lines from the real init.lua so
    # we run NvGuy's actual lazy.setup spec without the press-ENTER prompt.
    real_init = (config / "nvim" / "init.lua").read_text()
    bootstrap_init = "\n".join(
        line for line in real_init.splitlines()
        # Skip the two lines that dofile() the base46 cache before it exists.
        if not line.lstrip().startswith("dofile(vim.g.base46_cache")
    )
    # Append: explicitly wait for Lazy install to finish, then prime base46.
    # We loop until lazy reports zero pending tasks; sync({wait=true}) and
    # ":Lazy! sync" both have edge cases where new clones scheduled during
    # the call (e.g., dependencies discovered mid-flight) aren't awaited.
    bootstrap_init += (
        "\n-- guyide e2e bootstrap additions --\n"
        'local lazy = require("lazy")\n'
        '-- First pass: install everything declared.\n'
        'lazy.sync({ wait = true, show = false })\n'
        '-- Drain follow-up tasks (dependencies of dependencies, etc.).\n'
        'for _ = 1, 20 do\n'
        '  local stats = lazy.stats()\n'
        '  if stats.startuptime and stats.count == stats.loaded + stats.not_loaded then\n'
        '    -- still need to verify nothing pending; iterate plugins:\n'
        '  end\n'
        '  local pending = false\n'
        '  for _, p in ipairs(lazy.plugins()) do\n'
        '    if p._.installed == false then pending = true break end\n'
        '  end\n'
        '  if not pending then break end\n'
        '  lazy.install({ wait = true, show = false })\n'
        'end\n'
        # Final sanity: bail loudly if anything is still missing.
        'local missing = {}\n'
        'for _, p in ipairs(lazy.plugins()) do\n'
        '  if p._.installed == false then table.insert(missing, p.name) end\n'
        'end\n'
        'if #missing > 0 then\n'
        '  error("guyide_prewarm: plugins still not installed after sync: " .. table.concat(missing, ", "))\n'
        'end\n'
        'pcall(function() require("base46").load_all_highlights() end)\n'
    )
    bootstrap = warm_root / "bootstrap.lua"
    bootstrap.write_text(bootstrap_init)

    print("\n[nvguy_prewarm] lazy install (this can take several minutes on cold cache)...", flush=True)
    _run(
        [
            "nvim", "--headless",
            "-u", str(bootstrap),
            "+qall!",
        ],
        env=env,
        timeout=900,
    )

    return {
        "config": config,
        "data": data,
        "state": state,
        "cache": cache,
        "vscodium_local": env.get("NVGUY_VSCODIUM_LOCAL"),
    }


@pytest.fixture
def guyide_stack(
    tmp_path: Path,
    guyide_bin: Path,
    nvguy_prewarm: dict[str, Path],
) -> Stack:
    """Stand up a fully isolated tmux+nvim stack for one test."""
    if not shutil.which("tmux"):
        pytest.skip("tmux not on PATH")
    if not shutil.which("nvim"):
        pytest.skip("nvim not on PATH")

    pid = os.getpid()
    sock_name = f"guyide-e2e-{pid}-{int(time.time() * 1000) % 1_000_000}"
    session = "e2e"

    workdir = tmp_path / "project"
    workdir.mkdir()
    shutil.copy(E2E_DIR / "sample.py", workdir / "sample.py")
    (workdir / ".vscode").mkdir()
    shutil.copy(E2E_DIR / ".vscode" / "launch.json", workdir / ".vscode" / "launch.json")

    xdg_config = tmp_path / "xdg-config"
    xdg_data = tmp_path / "xdg-data"
    xdg_state = tmp_path / "xdg-state"
    xdg_cache = tmp_path / "xdg-cache"
    runtime = tmp_path / "runtime"
    # Unix sockets are limited to ~104 bytes on macOS. pytest tmp_path can
    # easily exceed that, so place the nvim socket under /tmp instead.
    short_runtime = Path(f"/tmp/guyide-e2e-{pid}-{int(time.time() * 1000) % 1_000_000}")
    short_runtime.mkdir(parents=True, exist_ok=True)
    short_runtime.chmod(0o700)
    for p in (runtime,):
        p.mkdir(parents=True)
    runtime.chmod(0o700)

    # Snapshot the warmed-up XDG dirs so each test gets a fresh, but
    # already-installed, environment. copytree is fine on tmpfs; on slower
    # disks a future optimization is hard-linking the data dir.
    shutil.copytree(nvguy_prewarm["config"], xdg_config, symlinks=True)
    shutil.copytree(nvguy_prewarm["data"], xdg_data, symlinks=True)
    shutil.copytree(nvguy_prewarm["state"], xdg_state, symlinks=True)
    shutil.copytree(nvguy_prewarm["cache"], xdg_cache, symlinks=True)

    env = {
        **os.environ,
        "HOME": str(tmp_path),  # contain plugin install/cache to tmp
        "XDG_CONFIG_HOME": str(xdg_config),
        "XDG_DATA_HOME": str(xdg_data),
        "XDG_STATE_HOME": str(xdg_state),
        "XDG_CACHE_HOME": str(xdg_cache),
        "XDG_RUNTIME_DIR": str(runtime),
        "TMUX": "",  # ensure tmux nests cleanly
    }
    if nvguy_prewarm.get("vscodium_local"):
        env["NVGUY_VSCODIUM_LOCAL"] = nvguy_prewarm["vscodium_local"]
    debugpy_python = os.environ.get("GUYIDE_E2E_DEBUGPY_PYTHON") or _detect_debugpy_python()
    if debugpy_python:
        env["NVGUY_DEBUGPY_PYTHON"] = debugpy_python

    # Wrapper script ensures the tmux child inherits our XDG/HOME overrides
    # regardless of how the runner's tmux server was started.
    vscodium_export = ""
    if nvguy_prewarm.get("vscodium_local"):
        vscodium_export = f"export NVGUY_VSCODIUM_LOCAL='{nvguy_prewarm['vscodium_local']}'\n"
    debugpy_export = ""
    if env.get("NVGUY_DEBUGPY_PYTHON"):
        debugpy_export = f"export NVGUY_DEBUGPY_PYTHON='{env['NVGUY_DEBUGPY_PYTHON']}'\n"
    wrapper = tmp_path / "run-nvim.sh"
    wrapper.write_text(
        "#!/usr/bin/env bash\n"
        "set -u\n"
        f"export HOME='{tmp_path}'\n"
        f"export XDG_CONFIG_HOME='{xdg_config}'\n"
        f"export XDG_DATA_HOME='{xdg_data}'\n"
        f"export XDG_STATE_HOME='{xdg_state}'\n"
        f"export XDG_CACHE_HOME='{xdg_cache}'\n"
        f"export XDG_RUNTIME_DIR='{short_runtime}'\n"
        f"{vscodium_export}"
        f"{debugpy_export}"
        f"cd '{workdir}'\n"
        f"exec nvim --listen \"$XDG_RUNTIME_DIR/nvim.guyide.0\" sample.py\n"
    )
    wrapper.chmod(0o755)
    _tmux(sock_name, "new-session", "-d", "-s", session, "-x", "200", "-y", "50", str(wrapper))

    # Wait for nvim socket to appear and accept connections.
    expected_sock = short_runtime / "nvim.guyide.0"

    def _sock_ready() -> bool:
        if not expected_sock.exists():
            return False
        with socket.socket(socket.AF_UNIX) as s:
            s.settimeout(0.5)
            s.connect(str(expected_sock))
            return True

    try:
        _wait_for(_sock_ready, timeout=120.0, interval=0.5, what="nvim socket (lazy install may take a while)")
    except TimeoutError:
        # Capture the pane so the user can see what happened.
        cap = _tmux(sock_name, "capture-pane", "-t", f"{session}:0", "-p", "-S", "-200", check=False)
        _tmux(sock_name, "kill-server", check=False)
        pytest.fail(f"nvim socket never appeared. tmux pane:\n{cap.stdout}")

    pane_id = _tmux(sock_name, "list-panes", "-t", f"{session}:0", "-F", "#{pane_id}").stdout.strip()

    stack = Stack(
        guyide=guyide_bin,
        tmux_socket=sock_name,
        session=session,
        nvim_pane=pane_id,
        nvim_sock=expected_sock,
        workdir=workdir,
        xdg_config_home=xdg_config,
        xdg_data_home=xdg_data,
        runtime_dir=runtime,
    )

    # Wait until nvim-launch.debug-rpc is loadable (depends on lazy install
    # of vscodium.nvim + nvim-dap).
    def _debugrpc_loaded() -> bool:
        proc = subprocess.run(
            [
                str(guyide_bin),
                "--socket",
                str(expected_sock),
                "--json",
                "nvim",
                "eval",
                "luaeval('pcall(require, \"nvim-launch.debug-rpc\")')",
            ],
            text=True,
            capture_output=True,
            timeout=10,
        )
        if proc.returncode != 0:
            return False
        try:
            return json.loads(proc.stdout).get("value") is True
        except Exception:
            return False

    try:
        _wait_for(_debugrpc_loaded, timeout=180.0, interval=1.0, what="nvim-launch.debug-rpc loadable")
    except TimeoutError:
        cap = _tmux(sock_name, "capture-pane", "-t", pane_id, "-p", "-S", "-200", check=False)
        _tmux(sock_name, "kill-server", check=False)
        pytest.fail(f"nvim-launch.debug-rpc never loaded.\nPane:\n{cap.stdout}")

    yield stack

    # Teardown: capture pane on failure for diagnostics, then kill server.
    if os.environ.get("GUYIDE_E2E_KEEP"):
        return
    _tmux(sock_name, "kill-server", check=False)
    shutil.rmtree(short_runtime, ignore_errors=True)
