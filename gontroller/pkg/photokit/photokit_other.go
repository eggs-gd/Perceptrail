//go:build !darwin

package photokit

// Authorize: no PhotoKit here
func Authorize() bool { return false }

// RunMain waits until done (nothing to serve)
func RunMain(done <-chan struct{}) { <-done }

func image(string, int, bool) ([]byte, error)         { return nil, ErrUnavailable }
func original(string) ([]byte, string, string, error) { return nil, "", "", ErrUnavailable }
func video(string, int) (string, error)               { return "", ErrUnavailable }
func live(string) error                               { return ErrUnavailable }
