# Changelog

## 0.3.2 — 2019-02-24

- Fix claws crashing on cmder

## 0.3.1 — 2018-08-12

### Fixed

- Fix `CLAWS_CONNECTION` always having the same value as `CLAWS_SESSION`

## 0.3.0 — 2018-03-04

### Added

- Message piping allows you to log messages that you receive and you send, or do any kind of preprocessing before being sent or being shown to you. You can check the README for more information.

### Changed

- We changed the JSON parsing library, so now we have more control over JSON formatting and we can keep the order of keys without sorting them alphabetically as we did before. We also collapse single-key arrays and objects so that they are on one line.
- Double-Esc does not work anymore to exit; it is encouraged to use Ctrl-C instead.

### Fixed

- Claws doesn't complain anymore when it can't find the config file. Mostly a quality-of-life change for new users.

## 0.2.0 — 2017-06-26

### Added

- Browse the log output by using your arrow keys, PgUp, PgDown, your mouse wheel and Home/End while in esc mode.
  - If you press one of the keys mentioned above, while in ESC mode the log will not autoscroll. To enable autoscroll again, simply press Ins to go in normal mode.

### Changed

- While in ESC mode, a cursor will be shown to indicate what the current scrolling position is.

## 0.1.0 — 2017-06-25

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
