# chain

Steps that run concurrently, connected by typed pipes. A step's logic is plain Go
(a `Decorator`, a `Router`, a `Spreader`, a `Consumer`); the package runs it: a goroutine per step,
reading one pipe and writing another, until the context ends.

```go
walks, in := chain.NewPipe[struct{}](0), chain.NewPipe[Raw](0)
parsed, out := chain.NewPipe[Parsed](0), chain.NewPipe[Item](100)

c := chain.New(errch)                           // errors of every step (skips never get here)
c.AddStep(chain.Spread(walks, in, walker))      // a request in, a batch out, then a flush
c.AddStep(chain.Parallel(4, in, parsed, parse)) // the same Decorator on 4 workers
c.AddStep(chain.Decorate(parsed, out, enrich))
c.AddStep(chain.End(out, publish))              // the end: every value consumed
go c.Process(ctx)                               // runs until ctx ends

walks.Send(ctx, struct{}{}) // the owner starts a batch, when it wants
<-c.Done()                  // the batch's flush reached every end
```

## Pipes and the flush

A `Pipe[T]` carries values and a **flush**. A `Spread` flushes after each value it
spread (a walk of a library). Every step passes the flush on **after the values
before it**, so a flush that reaches the end of a chain means every value of the
batch has gone through every step. **`Chain.Done()`** fires once the flush has
reached every `End` of the chain; the chain's owner waits on it (to repeat a walk
after a pause, say). The chain keeps running: the owner can `Send` into any pipe at
any time, and `Pipe.Flush` sends a flush from outside (a test driving a chain value
by value).

- **A `Flusher`** (`Flush() ([]To, error)`, optional on a step's logic) gives what it
  holds — a group not complete yet — and that goes out before the flush.
- **The barrier.** The steps that write to a pipe register as its writers when they
  are built. Where branches join (several steps writing one pipe), the reader passes
  the flush on once **every writer** has flushed. `Pipe.Send` (a value from outside
  the chain) is not a writer.
- **`Route`** sends a value to one output and a flush to every output.
- **`Parallel`** waits until every value before the flush is done before passing it
  on.

## Steps

| Constructor | Logic | What it does |
|---|---|---|
| `Decorate(in, out, d)` | `Decorator[Ti, To]`: `Decorate(Ti) (To, error)` | one value in, one out |
| `Parallel(n, in, out, d)` | the same `Decorator`, safe for concurrent use | n workers; the order may change |
| `Route(in, outs, r)` | `Router[T]`: `Route(T) (int, error)` | a value to one output (an index) |
| `Spread(in, out, s)` | `Spreader[Ti, To]`: `Spread(ctx, Ti, emit) error` | one value in, many out, then a flush |
| `End(in, c)` | `Consumer[T]`: `Consume(T) error` | a chain's end: every value consumed; counts for `Done` |
| `New(errch)` + `AddStep` | — | a chain; it is a step too (a sub-chain) |

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

`chain_test.go`, run with `-race`: a `Spread` batch and `Done`, a `Flusher`'s values
go before the flush, `Route` and the barrier at a join, `Parallel` waits for the
values in flight, errors and skips, a sub-chain inherits the error channel, cancel
unblocks a blocked send and `Stop` is called once.
