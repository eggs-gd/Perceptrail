# chain

Steps that run concurrently, connected by channels. A step's logic is plain Go (a
`Source`, a `Decorator`, a `Router`, a `Consumer`); the package runs it: a goroutine
per step, reading one channel and writing another.

```go
in, parsed, out := make(chan Raw), make(chan Parsed), make(chan Item, 100)

c := chain.New(errch)                           // errors of every step (skips never get here)
c.AddStep(chain.Entry(in, walker))              // the one input: its values, then it is done
c.AddStep(chain.Parallel(4, in, parsed, parse)) // the same Decorator on 4 workers
c.AddStep(chain.Decorate(parsed, out, enrich))
c.AddStep(chain.End(out, publish))              // the end: every value consumed
c.Process(ctx)                                  // one pass: returns when every step has
```

## A pass

A chain has **one input, its `Entry`** (an output only). A pass is `Process`: the
entry emits its values and returns, and its output closes. A step reads its input
until it closes, gives what it holds (a `Flusher`: `Flush() ([]To, error)` — a group
not complete yet) and returns. **An output closes once every step that writes to it
has returned**: the chain keeps a `sync.WaitGroup` per output, one count per writer —
where branches join (several steps writing one channel), the joint closes after the
last of them. `Process` returns when every step has, so every value of the pass went
through every step by then. Channels close once: the next pass is a new chain, with
new channels.

- **`Route`** sends a value to one output; its outputs close when it returns.
- **`Parallel`** returns after its last worker: every value done.
- **A sub-chain** closes its own outputs; a channel is written by steps of one chain.

## Steps

| Constructor | Logic | What it does |
|---|---|---|
| `Entry(out, s)` | `Source[T]`: `Start(ctx, emit) error` | the chain's one input |
| `Decorate(in, out, d)` | `Decorator[Ti, To]`: `Decorate(Ti) (To, error)` | one value in, one out |
| `Parallel(n, in, out, d)` | the same `Decorator`, safe for concurrent use | n workers; the order may change |
| `Route(in, outs, r)` | `Router[T]`: `Route(T) (int, error)` | a value to one output (an index) |
| `End(in, c)` | `Consumer[T]`: `Consume(T) error` | a chain's end: every value consumed |
| `New(errch)` + `AddStep` | — | a chain; it is a step too (a sub-chain); `Process` is a pass |

Optional on any logic:

- **`Stopper`** (`Stop()`): called once, when the step ends.
- **`Flusher[To]`**: what the step holds goes out when its input ends.

## Errors

- **A step's error** goes to the chain's error channel.
- **`ErrSkippedItem`** (a value dropped on purpose: buffered, unchanged, not wanted)
  is **not an error**: it is dropped and never reaches the channel.
- **A sub-chain** made with `New(nil)` uses the error channel of the chain it runs in.
- **Every send** (to a channel or to the error channel) gives up when the context
  ends, so a stopped chain never blocks; a step stopped by the context does not
  flush.

## Tests

`chain_test.go`, run with `-race`: a pass, a `Flusher`'s values before its output
closes, `Route` and a join that closes after its slow branch, `Parallel` finishes its
values, errors and skips, a sub-chain inherits the error channel, cancel unblocks a
blocked send and `Stop` is called once.
