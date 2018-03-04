package nanojson

import (
	"sync"
)

/*
Pools are sync.Pools used to efficiently reuse data structures that would
otherwise escape to heap (and require to be allocated every time, thus
leading to overall slowness of the package.) Normally, users of the package
don't need to fine-tune this, however if you have particular needs it might
come in handy to change some of them.

ValueSlice

ValueSlice is used when creating children elements to a Value - which is to
say when there is an object or an array. Since ValueSlice enters the domain of
the user, Children slices are not automatically returned to the pool - however
they will be if the user practices good hygene and calls Recycle on the root
value once it's done dealing with the parsed JSON data. Recycling greatly
improves the speed of parsing.

The cap of the slices in the pool is vital. When parsing a JSON object or array,
nanojson will cycle through the children elements and will append values to the
slice, as long as the cap is not reached. Once len == cap, the parser will give
back the slice to the pool, and will switch to use append to grow and add new
elements to the slice. This, of course, incurs in a costly memory allocation.

By default, the pool always returns a []Value of size 1024 - on the assuption
that most APIs often return less than (or equal to) that in arrays and objects.
The downside of this is that even a simple [1,2,3] takes up 120kb of data
(on 64-bit systems). This may seem disastrous, especially after considering a
potentially dangerous payload like the following Python code:

	"[" + ("[" * 40 + "1337" + ",[[1337]]]" * 40 + ",") * 500 + "[[7331]]" + "]"

However, you should not forget that in modern times our machines have virtual
RAM which can help handle such abuses of memory - so you should probably not
worry about exceeding of the physical RAM.

If you want to replace ValueSlice and want to have an estimate of how memory a
[]Value takes, it's unsafe.Sizeof(Value{}) * cap.

EncodeStateBuf

When calling Encode on a value, EncodeStateBuf is called to retrieve a []byte
of size 255. (note: for the moment it MUST be 255, no other size is allowed.)
The buffer is used mostly to batch calls to Write - the change showed roughly a
0.75x improvement in speed in our benchmarks, although it did place more strain
on the encoder rather than the writer.
*/
var Pools = struct {
	ValueSlice     *sync.Pool
	EncodeStateBuf *sync.Pool
	Value          *sync.Pool
	PropertyMap    *sync.Pool
}{
	ValueSlice: &sync.Pool{
		New: func() interface{} {
			return make([]Value, 0, 1024)
		},
	},
	EncodeStateBuf: &sync.Pool{
		New: func() interface{} {
			return make([]byte, 255)
		},
	},
	Value: &sync.Pool{
		New: func() interface{} {
			return &Value{}
		},
	},
	PropertyMap: &sync.Pool{
		New: func() interface{} {
			return make(map[string]int)
		},
	},
}
