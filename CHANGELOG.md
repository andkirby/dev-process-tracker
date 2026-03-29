# Changelog

## 0.2.2

- Added a Shift+S sort direction toggle in the TUI so sort order can be reversed without changing the active column
- Fixed managed service PID validation so stop and restart only act on processes that still match the registered service
- Fixed cross-platform builds by separating Unix and Windows process control paths

## 0.2.1

- Added table sorting controls with mouse support and reverse sort in the TUI

## 0.2.0

- Added multi-service `start`, `stop`, and `restart` commands with quoted glob pattern support so multiple managed services can be controlled in one invocation
- Added `name:port` targeting for managed services so ambiguous service names can be disambiguated from the CLI
- Extracted the Bubble Tea UI into `pkg/cli/tui` so the TUI logic is isolated from the main CLI package
- Added mouse row selection, mouse wheel scrolling, and viewport-focused navigation so table and log interaction works without keyboard-only control
- Added centered modal overlays for help and confirmation dialogs so help and destructive actions no longer replace the main table view
- Replaced the ad hoc search field with Bubbles text input so filter editing behaves like a real input control and updates inline in the footer
- Simplified the table chrome by moving counts into headers, bolding the active sort column, and removing redundant status text from the top of the screen
- Fixed `Enter` handling so the top section opens logs and the bottom section starts the selected managed service without being swallowed by confirm bindings
- Fixed log rendering so the header is separated from the first log line and the viewport uses the actual remaining terminal height
- Fixed stale table layout offsets so footer spacing, viewport sizing, and mouse hit-testing stay aligned after the filter moved into the footer
- Added shared keymap-driven help text with Bubble components so visible shortcuts and actual bindings stay in sync
- Added clearer TUI and quickstart documentation so the current footer filter, modal help, mouse controls, batch commands, and logs header behavior are documented
- Bumped the application version to `0.2.0` and rendered the version in the TUI header in muted gray
