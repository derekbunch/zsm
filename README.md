# zsm

`zsm` is a small terminal session manager for [Zellij](https://zellij.dev/) backed by [zoxide](https://github.com/ajeetdsouza/zoxide). It gives you one fuzzy-searchable list of live Zellij sessions, resurrectable sessions, and frequently used directories, then attaches to an existing session or creates one in the selected directory.

## Demo

https://github.com/user-attachments/assets/a7d2f5ff-ee25-40ad-a1a6-d012054261f9

## Features

- Fuzzy search across Zellij sessions and zoxide directories.
- Attach to live sessions, resurrect exited sessions, or create a new session from a directory.
- In-Zellij session switching when `zsm` is launched inside an existing Zellij session.
- Layout previews for live and exited sessions.
- Directory previews through a configurable command such as `eza`, `tree`, or `lsd`.
- Rename, terminate, and delete sessions from the UI.
- Delete, bump, and demote zoxide entries from the UI.
- Light/dark terminal-aware colors with TOML theme overrides.

## Requirements

- Go 1.26 or newer.
- `zellij` available on `PATH`.
- `zoxide` available on `PATH`.
- A Nerd Font for the icons.
- Optional: `eza` for the default directory preview command.

## Install

```sh
go install github.com/derekbunch/zsm@latest
```

For local development:

```sh
git clone https://github.com/derekbunch/zsm.git
cd zsm
go install .
```

## Usage

Run:

```sh
zsm
```

Use a specific Zellij layout when creating new sessions:

```sh
zsm -layout compact
```

Fill the terminal height, useful from a floating pane:

```sh
zsm --float
```

Set `ZSM_DEBUG` to write Bubble Tea debug logs without corrupting the TUI:

```sh
ZSM_DEBUG=/tmp/zsm.log zsm
```

## Keybindings

| Key | Action |
| --- | --- |
| `enter` | Attach to the selected session, resurrect an exited session, or create a session for a directory |
| `esc` | Clear the filter, then quit when the filter is empty |
| `ctrl+c` | Quit |
| `ctrl+l` | Show the selected item's full path |
| `ctrl+w` | Increment the selected zoxide entry's score |
| `ctrl+x` | Decrement the selected zoxide entry's score |
| `ctrl+d` | Delete the selected session or zoxide entry |
| `ctrl+t` | Terminate a live session but keep resurrection data |
| `ctrl+r` | Rename a live session |
| `ctrl+e` | Create/attach with a custom session name for a directory |
| `up`, `ctrl+p` | Move up |
| `down`, `ctrl+n` | Move down |
| `ctrl+k`, `ctrl+j` | Scroll preview up/down |
| `ctrl+b`, `ctrl+f` | Page preview up/down |

## Configuration

`zsm` reads `~/.config/zsm/config.toml`. If the file is missing or invalid, built-in defaults are used.

Start from the example:

```sh
mkdir -p ~/.config/zsm
cp examples/config.toml ~/.config/zsm/config.toml
```

Important options:

```toml
layout = "zjstatus"
height = 15
width = 0
preview_position = "right"
show_score = true

[preview]
command = "eza"
options = ["-T", "-L2", "--color=always", "--icons=always"]
ignore_flag = "-I"
ignore_separator = "|"
ignore = [".git", "node_modules", ".venv", ".DS_Store"]
```

Set `preview_position = "none"` to disable previews. To use another preview tool, set `preview.command` and `preview.options`; `zsm` appends the selected directory as the last argument.

## Publishing

Before cutting a release, verify the module and build:

```sh
go test ./...
go install .
```

Then tag and push:

```sh
git tag v0.1.0
git push origin master --tags
```

After the tag is visible on GitHub, users can install it with `go install github.com/derekbunch/zsm@latest`.

## License

MIT
