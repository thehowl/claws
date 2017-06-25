# Changelog

## Unreleased

## 0.1.0 -- 2017-06-25

First official version of claws.

### Added

- Command line interface for claws.
- ESC mode, through which you can do special actions by pressing ESC then a key.
- Ability to exit claws by pressing ESC in ESC mode.
- JSON formatting in messages received by server, toggleable with `j` in ESC mode.
- Timestamps in messages on the log output, toggleable with `t` in ESC mode.
- Insert/normal mode by pressing Ins, replace mode by pressing Ins in normal mode already.
- Ability to enter normal mode by pressing `i` in ESC mode.
- Ability to enter replace mode by pressing `R` in ESC mode.
- Connect to a WebSocket by passing a ws:// or wss:// URL as the first argument on the command line.
- Connect to a WebSocket by pressing `c` in ESC mode and then writing down the ws:// or wss:// URL.
- Quit a websocket connection by pressing `q` in ESC mode.
- Add welcome screen that is shown if no argument is passed on the command line.
- Ability to show the welcome screen by pressing `h` in ESC mode.
- Start creating releases for claws, automatically built using [goreleaser](https://github.com/goreleaser/goreleaser), and document changes in CHANGELOG.md.
