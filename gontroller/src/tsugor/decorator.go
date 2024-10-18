// Threadind sug(a/O)r
package tsugor

import "sync"

func decorate[Ti any, To any](count int, chin <-chan Ti, chout chan<- To, fn func(input Ti) To, extWg *sync.WaitGroup) {
	var intWg = sync.WaitGroup{}

	if extWg != nil {
		defer extWg.Done()
	}

	for i := 0; i < count; i++ {
		intWg.Add(1)
		go func() {
			defer intWg.Done()
			for input := range chin {
				chout <- fn(input)
			}
		}()
	}

	intWg.Wait()
	close(chout)
}

// Take raw data from first channel and put decarodated data to second.
// After input channel will be closed externally - complete job and close output channel also
//
// Parameters:
// - [Ti, To]: type in type out
// - count: amount of workers
// - chin: input channel, moderates outside, readonly for decorator
// - chout: output channel, given from outside but will be closed inside after job done
// - fn: converter from in data to out data, takes input, returns converted
func DecorateSync[Ti any, To any](count int, chin <-chan Ti, chout chan<- To, fn func(input Ti) To) {
	decorate(count, chin, chout, fn, nil)
}

// Take raw data from first channel and put decarodated data to second.
// After input channel will be closed externally - complete job and close output channel also
//
// Parameters:
// - [Ti, To]: type in type out
// - count: amount of workers
// - chin: input channel, moderates outside, readonly for decorator
// - chout: - output channel, given from outside but will be closed inside after job done
// - fn: converter from in data to out data, takes input, returns converted
// - extWg: if runs as goroutine itself, provide external WaitGroup to proper handling of result
func DecorateAsync[Ti any, To any](count int, chin <-chan Ti, chout chan<- To, fn func(input Ti) To, extWg *sync.WaitGroup) {
	decorate(count, chin, chout, fn, extWg)
}
