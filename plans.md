# zsm plans

## Future improvements

- [ ] Split sessions and directories into separate panels (like lazygit) instead of a combined list
  - [ ] Tab or arrow keys to switch focus between panels
  - [ ] Each panel has its own filter/search
  - [ ] Sessions panel could show status (active pane count, last attached, etc.)
- [ ] Session preview using `zellij action dump-screen` for active sessions
- [ ] Directory preview using native file tree (no eza dependency)
- [ ] Config file (toml?) for:
  - [ ] Default layout per directory pattern
  - [ ] Custom session names per project
  - [ ] Startup commands
  - [ ] Pinned/favorite sessions
- [ ] Sort directories by frecency score (parse zoxide db directly)
- [ ] Session groups/tags
- [ ] Quick-switch keybind to bounce between last two sessions (like `cd -`)
