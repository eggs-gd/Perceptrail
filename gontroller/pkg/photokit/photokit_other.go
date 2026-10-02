//go:build !darwin

package photokit

// Authorize: no PhotoKit here
func Authorize() bool { return false }

// RunMain waits until done (nothing to serve)
func RunMain(done <-chan struct{}) { <-done }

func image(string, int) error { return ErrUnavailable }
func video(string, int) error { return ErrUnavailable }
func live(string) error       { return ErrUnavailable }
