package discover

import (
	"testing"
	"time"

	"perceptrail/gontroller/pkg/importer/flow"
)

// The walker walks again only after the previous walk's work is done
func TestWalkerRepeatsAfterIdle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/a.jpg")

	m := newTestMonitor(t, root)
	m.interval = time.Millisecond
	m.progress = flow.NewProgress()
	chin := make(chan inType)
	go m.Start(chin, m.ctx)

	walks := 0
	deadline := time.After(5 * time.Second)
	for walks < 2 {
		select {
		case in := <-chin:
			if in.done == nil {
				continue
			}
			walks++
			if walks == 1 {
				// A group of the walk is still being processed: no second walk yet
				m.progress.Passed()
				m.progress.WalkGated()
				select {
				case in := <-chin:
					t.Fatalf("walked again while busy: %+v", in)
				case <-time.After(50 * time.Millisecond):
				}
				m.progress.Finished()
			} else {
				m.progress.WalkGated()
			}
		case <-deadline:
			t.Fatalf("only %d walks", walks)
		}
	}
}
