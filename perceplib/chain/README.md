# chain

Steps that run concurrently, connected by typed pipes. A step's logic is plain Go
(a `Source`, a `Decorator`, a `Router`, a `Consumer`); the package runs it: a goroutine per step,
reading one pipe and writing another, until the context ends.

```go
in, parsed, out := chain.NewPipe[Raw](0), chain.NewPipe[Parsed](0), chain.NewPipe[Item](100)

c := chain.New(errch)                           // errors of every step (skips never get here)
c.AddStep(chain.Entry(in, walker))              // the one input: a pass of it, then a flush
c.AddStep(chain.Parallel(4, in, parsed, parse)) // the same Decorator on 4 workers
c.AddStep(chain.Decorate(parsed, out, enrich))
c.AddStep(chain.End(out, publish))              // the end: every value consumed
go c.Process(ctx)                               // the steps run until ctx ends

for c.Run(ctx) { // a pass: returns once its flush reached every end
    time.Sleep(time.Minute)
}
```

## Pipes and the flush

A chain has **one input, its `Entry`**: an output only, started by the chain. A
pass (`Chain.Run`) starts it; after its values it flushes. A `Pipe[T]` carries
values and the **flush**. Every step passes the flush on **after the values before
it**, so a flush that reaches the end of a chain means every value of the pass has
gone through every step: `Run` returns then (the chain counts its `End`s). The steps
keep running between passes; the owner decides when the next one comes.

- **A `Flusher`** (`Flush() ([]To, error)`, optional on a step's logic) gives what it
  holds — a group not complete yet — and that goes out before the flush.
- **The barrier.** The steps that write to a pipe register as its writers when they
  are built. Where branches join (several steps writing one pipe), the reader passes
  the flush on once **every writer** has flushed.
- **`Route`** sends a value to one output and a flush to every output.
- **`Parallel`** waits until every value before the flush is done before passing it
  on.

## Steps

| Constructor | Logic | What it does |
|---|---|---|
| `Decorate(in, out, d)` | `Decorator[Ti, To]`: `Decorate(Ti) (To, error)` | one value in, one out |
| `Parallel(n, in, out, d)` | the same `Decorator`, safe for concurrent use | n workers; the order may change |
| `Route(in, outs, r)` | `Router[T]`: `Route(T) (int, error)` | a value to one output (an index) |
| `Entry(out, s)` | `Source[T]`: `Start(ctx, emit) error` | the chain's one input: a pass, then a flush |
| `End(in, c)` | `Consumer[T]`: `Consume(T) error` | a chain's end: every value consumed; a pass ends at the ends |
| `New(errch)` + `AddStep` | — | a chain; it is a step too (a sub-chain); `Process` runs the steps, `Run` one pass |

Optional on any logic:

- **`Stopper`** (`Stop()`): called once, when the step ends.
- **`Flusher[To]`**: what the step holds goes out on a flush.

## Errors

- **A step's error** goes to the chain's error channel.
- **`ErrSkippedItem`** (a value dropped on purpose: buffered, unchanged, not wanted)
  is **not an error**: it is dropped and never reaches the channel.
- **A sub-chain** made with `New(nil)` uses the error channel of the chain it runs in.
- **Every send** (to a pipe or to the error channel) gives up when the context ends,
  so a stopped chain never blocks.

## Tests

`chain_test.go`, run with `-race`: passes (`Entry`, `Run`), a `Flusher`'s values
go before the flush, `Route` and the barrier at a join, `Parallel` waits for the
values in flight, errors and skips, a sub-chain inherits the error channel, cancel
unblocks a blocked send (a cancelled pass is not done) and `Stop` is called once.
