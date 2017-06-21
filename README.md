# claws

an Awesome WebSocket CLient.

[![asciicast](https://asciinema.org/a/125778.png)](https://asciinema.org/a/125778)

Websockets have been on the rise for a long time, but there is no good command line client that allows to interface with websockets without having to deal with barebones interfaces. Claws aims to make testing websockets and interfacing with them easier and more pleasant.

## Getting started

For the moment, this is in a very early stage, and as such it does not have binaries you can install directly. But if you have go, it is easy as running the following command and making sure that `$GOPATH/bin` (or `$GOBIN`) is in your PATH.

```
go get -v -u github.com/thehowl/claws
```

## Usage

```
claws [wsURL]
```

wsURL is an optional websocket URL to connect to once the UI has been initialised.

Once the UI has been initialised, you will be by default in "normal mode": green box with nothing in it. This means you are composing a message to send to the server through the websocket. By pressing `Esc`, followed by a letter, you can do a variety of actions explained in the following table.

Letter   | Meaning
---------|----------------------------------------------------
`Esc`    | Close the application. (Press Esc two times)
`c`      | Create a new WebSocket connection. Will prompt for an URL. If nothing is passed, previous WebSocket URL will be used.
`q`      | Close current WebSocket connection.

If you need to go back to normal mode while in Esc mode, simply press Insert or `i`.

This is probably what you will need most of the time. If you're looking to have more things, [move on](#advanced-usage).

When you're typing text into the field, you can browse through the history of previous text, even in previous sessions, in a bash-like fashion.

### Advanced usage

Claws stores its configuration file in `~/.config/claws.json`. You are welcome to hack it and change values to how you see fit.

There are also more actions you can activate using ESC + key, that are generally not used on a day-to-day basis.

Letter   | Meaning
---------|----------------------------------------------------
`t`      | Toggle timestamps before messages in console.
`j`      | Toggle auto-detection of JSON in server messages and automatic tab indentation.
`h`      | View help/welcome screen with quick commands.
`i`      | Go into insert mode (can also be done by pressing Insert).
`R`      | Go into replace/overtype mode (can also be done by pressing Insert a couple of times).

## Caveat emptor

claws is software in early development. It may often not work as intended. Whenever you see something odd, or you have a proposal to make, please open up an issue for discussion.
