package render

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
)

// proc: how render starts its tools — below the server and the rest of the host
// (nice, when the host has it: the gallery, an Immich beside us answer first), each
// on its share of the CPUs (the CPUs over the workers: N workers never ask for more
// than the machine, a tool's own threads included)
type proc struct {
	nice    string // the nice command; "": the tools run at the server's priority
	threads int    // a tool's threads
}

// niceness: render's priority below the server's (0 the server's, 19 the lowest)
const niceness = "10"

func newProc(workers int) proc {
	p := proc{threads: max(runtime.NumCPU()/workers, 1)}
	if nice, err := exec.LookPath("nice"); err == nil {
		p.nice = nice
	}
	return p
}

// command: a tool's process, killed with ctx — nice execs the tool, so the kill
// reaches it. libvips reads its threads from the environment, ffmpeg from its
// arguments (threads).
func (p proc) command(ctx context.Context, name string, args ...string) *exec.Cmd {
	if p.nice != "" {
		args = append([]string{"-n", niceness, name}, args...)
		name = p.nice
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "VIPS_CONCURRENCY="+strconv.Itoa(p.threads))
	return cmd
}

// threadArgs: ffmpeg's threads (before -i: the decoder's, after: the encoder's and
// the filters')
func (p proc) threadArgs() []string {
	return []string{"-threads", strconv.Itoa(p.threads)}
}
