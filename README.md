<p align="center">
<img src="https://random.zxq.co/claws.gif" alt="claws demonstration">
</p>
<h5 align="center">an Awesome WebSocket CLient</h5>
<h1 align="center">Claws <a href="https://travis-ci.org/thehowl/claws"><img src="https://travis-ci.org/thehowl/claws.svg?branch=master"></a></h1>

Claws is an interactive command line client to interface with WebSockets. It
allows you to clearly identify the messages you send and those you receive,
scroll through your history and send commands in a shell-like fashion,
format JSON messages, add timestamps and perform pre-formatting on messages
you send and receive.

## Getting started

You can grab the latest release binary from the [releases page](https://github.com/thehowl/claws/releases). Simply download it, extract it, and run it in a command line.

If you have Go set up, it is easy as running the following command and making sure that `$GOPATH/bin` (or `$GOBIN`) is in your PATH.
Generally, this should be pretty stable, but keep in mind that it builds on master, so it may break at any point.

```
go install -v howl.moe/claws@latest
```

## Usage

[A 4-minute video tutorial is available to explain the basics of the
interface](https://youtu.be/yIhEcA0Z794)

```
claws [wsURL]
```

wsURL is an optional websocket URL to connect to once the UI has been initialised.

The interface has some similar concepts to vim, but it should come off as more
intuitive (and it's also easier to quit - as Ctrl-c quits the program as you
would expect!).

Once the UI has been initialised, you will be by default in "insert mode":
green box with nothing in it. This means you are composing a message to send
to the server through the websocket. By pressing `Esc`, followed by a letter,
you can do a variety of actions explained in the following table.

Letter   | Meaning
---------|----------------------------------------------------
`i`      | Go to insert mode (also works by pressing the Ins key).
`c`      | Create a new WebSocket connection. Will prompt for an URL. If nothing is passed, previous WebSocket URL will be used.
`q`      | Close current WebSocket connection.

Extra keybindings using Ctrl are Ctrl-C, which quits the program, and Ctrl-L,
which clears the buffer (like the `clear` command in your command line)

If you want to scroll through the logs, while in Esc mode press the arrow keys,
PgUp/PgDown, Home/End. Keep in mind that pressing any of these will disable autoscroll, so new elements from the log won't be shown unless you scroll down.

When you're typing text into the field, you can browse through the history of previous text, even in previous sessions, in a bash-like fashion using the up and down keys.

### Advanced usage

There are also more actions you can activate using ESC + key, that are generally not used on a day-to-day basis.

Letter   | Meaning
---------|----------------------------------------------------
`t`      | Toggle timestamps before messages in console.
`j`      | Toggle auto-detection of JSON in server messages and automatic tab indentation.
`h`      | View help/welcome screen with quick commands.
`R`      | Go into replace/overtype mode (can also be done by pressing Insert a couple of times).

## Configuration

Claws stores its configuration file in `~/.config/claws.json`. You are welcome
to hack it and change values to how you see fit. Here's a list of the values.
Note that the path to the file is the same also on Windows.

* **Info:** this field is used to redirect readers to this documentation file.
* **JSONFormatting:** either true or false, depending on whether JSON formatting
  is enabled.
* **Timestamp:** a timestamp with which all messages to the console should be prefixed. The defaults can be toggled using the `t` key in esc mode, although you can also use your own prefix, following [Go's system of formatting dates](https://golang.org/pkg/time/#Time.Format). The default values are an empty string `""` or `"2006-01-02 15:04:05 "`.
* **LastWebsocketURL:** URL of the last websocket you connected to. Used when connecting using the `c` key without specifying an URL.
* **LastActions:** 50 most recent messages you sent to the console, used for seeking through history using up and down.

### Pipe

Piping allows you to log the messages you send and the messages you receive, or do any kind of pre-processing before they are sent or before they are shown on the console.

Pipes are specified by the `Pipe` configuration variable. It defaults to null - to set it, create an array containing in the first place a command and then the arguments for it. This should be a command available in your `$PATH` or an absolute path.

For UNIX system, this means you can do logging effectively using `tee`. The most basic form of logging may look like this:

```json
"Pipe": {
	"In": ["tee", "-a", "received_messages.log"],
	"Out": ["tee", "-a", "sent_messages.log"]
}
```

This will append to the given log files the received and sent messages. But it doesn't have to stop there! You can really create any script that may do any pre-processing you want to the messages you receive and those you send. If you intend to write a non-trivial script, here are things that might come useful to know:

* Any non-zero exit code will show an error on the console.
* The program should be pretty fast to run, as the message can't be shown until the processing has finished.
* There are some environment variables which provide information on the connection:
  * **`CLAWS_PIPE_TYPE`:** The type of pipe; either `in` or `out`.
  * **`CLAWS_SESSION`:** UNIX timestamp in microseconds of when the session was started.
  * **`CLAWS_CONNECTION`:** UNIX timestamp in microseconds of when the connection was started.
  * **`CLAWS_WS_URL`:** WebSocket URL we're connected to.

The sky is the limit here, so you can really do anything you can think of. Here are some examples (feel free to add more with a PR!):

* [New score notifier](https://gist.github.com/thehowl/97c77114859c64c67d357adf604229f4), using the [Ripple API](http://docs.ripple.moe/docs/api/websocket) (bash).

## Contributing

Claws is mostly feature-complete, though we have something that might interest you on our [issue list](https://github.com/thehowl/claws/issues). If, instead, you're interested in reporting a bug or asking for a new feature, you can create a new [issue](https://github.com/thehowl/claws/issues/new). There are no real contribution guidelines, but try to write some good Go code and use `go fmt` :).
