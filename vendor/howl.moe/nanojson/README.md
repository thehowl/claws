# nanojson [![pipeline status](https://gitlab.com/tyge/nanojson/badges/master/pipeline.svg)](https://gitlab.com/tyge/nanojson/commits/master) [![coverage report](https://gitlab.com/tyge/nanojson/badges/master/coverage.svg)](https://tyge.gitlab.io/nanojson/) [![GoDoc](https://godoc.org/howl.moe/nanojson?status.svg)](https://godoc.org/howl.moe/nanojson)

> _Parse JSON in nanoseconds, not microseconds_

A WIP JSON decoder/encoder for Go. At the moment parsing works, though it
requires a lot of manual work on the user end. Planned features:

- Encoding of Values
- Static code generation for marshaling/unmarshaling of types

## Why?

Yes, there are already plenty of JSON encoders/decoders for Go out there.
jsonparser has a [nice list of them,](https://github.com/buger/jsonparser#benchmarks)
and related benchmarks. As you can see, all except jsonparser require memory
allocations, and even for the small payload they take at least a microsecond to
complete.

nanojson is here to bring nanoseconds to json. (Read: make parsing usually take
less than a microsecond).

## Status

Very WIP.
