# Welcome to GuyIDE

You're in **NvGuy**, the neovim half of GuyIDE. Right now you're looking at a single nvim pane. To unlock the full terminal IDE experience, enter **IDE mode**.

---

## Enter IDE mode

Press **`Ctrl-a` then `e`**.

That's the tmux prefix (`Ctrl-a`) followed by `e`. It opens the 3-pane layout:

```
+---------------------------+--------------+
|                           |              |
|   nvim (this pane)        |   AI agent   |
|                           |              |
|---------------------------|              |
|   terminal                |              |
+---------------------------+--------------+
```

Once you're there, your AI agent can drive the debugger, read terminal output, and persist its conversation across restarts.

---

## Daily-driver keys

These are the five keybindings you'll use every session. Everything else can wait.

| Keys | What it does |
|---|---|
| `Ctrl-a e` | Enter IDE mode (3-pane layout) |
| `Ctrl-a d` | Detach from tmux — your session keeps running in the background. Reattach later with `tmux attach`. |
| `Ctrl-a Ctrl-r` | Restore the last saved session (layout + open files + AI conversation) |
| `Ctrl-a ?` | Show the full list of tmux commands |
| `Space m` | Open the neovim menu bar (only when this nvim pane is focused) |

> `Ctrl-a` means hold Ctrl and tap `a` once. Then release both keys before pressing the next key. It is the *prefix*, not a chord.

---

## What's installed

GuyIDE just placed three things on your machine:

- **NvGuy** at `~/.config/nvim` — your editor + debugger
- **tmux** config at `~/.tmux.conf` — your window manager
- **OpenCode** (or whichever agent you picked) — your AI pair

All of it lives under `~/.guyide/` and can be removed with `guyide uninstall`.

---

## Next steps

1. Press `Ctrl-a e` now to see the IDE layout.
2. Open a project: `cd ~/your-project && tmux attach` (or just `tmux` for a fresh session).
3. Ask your AI agent to debug something:
   > "Set a breakpoint at main.py:42 and tell me what `user_id` is when the test fails."

Run `guyide doctor` any time to check the health of your install.

Happy hacking.
