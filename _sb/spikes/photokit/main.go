//go:build darwin

// The PhotoKit spike: when Photos is asked (PhotoKit) for an image of an asset that
// is only in iCloud, does the rendition become local in the library — a file in
// resources/derivatives, the DB's local availability — or are we only handed the
// pixels? And how long does it take. Roadmap: "Apple Photos first — a spike".
//
// It only reads the library; Photos itself may download into it. Run it from your
// own terminal (the one that runs gontroller — it can read the library):
//
//	go build -o photokit-spike . && ./photokit-spike <asset UUID>...
package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework AppKit -framework Photos -framework AVFoundation -framework CoreMedia
#cgo LDFLAGS: -Wl,-sectcreate,__TEXT,__info_plist,${SRCDIR}/Info.plist
#include <stdlib.h>
#include "photokit.h"
*/
import "C"

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"
)

// main on the main thread: some PhotoKit results (Live Photos) come on the main
// queue, and the waits turn the main run loop for them
func init() { runtime.LockOSThread() }

var statuses = map[int]string{0: "not determined", 1: "restricted", 2: "denied", 3: "authorized", 4: "limited"}

func main() {
	home, _ := os.UserHomeDir()
	lib := flag.String("lib", filepath.Join(home, "Pictures", "Photos Library.photoslibrary"), "the Photos library")
	size := flag.Int("size", 2048, "the image asked for: at most size×size pixels (the viewer's medium)")
	wait := flag.Duration("wait", 10*time.Second, "how long to let Photos write its DB after the request")
	statusOnly := flag.Bool("status", false, "print the authorization status and stop (no prompt)")
	kind := flag.String("kind", "photo", "what to ask for: photo (an image), video (the video), live (a Live Photo)")
	flag.Parse()

	fmt.Printf("Photos access: %s\n", statuses[int(C.pk_status())])
	if *statusOnly {
		return
	}
	if st := int(C.pk_auth()); st != 3 && st != 4 {
		fmt.Printf("not authorized (%s): System Settings → Privacy & Security → Photos\n", statuses[st])
		os.Exit(1)
	}
	if flag.NArg() == 0 {
		fmt.Println("no asset UUIDs given")
		os.Exit(2)
	}

	for _, uuid := range flag.Args() {
		fmt.Printf("\n=== %s\n", uuid)
		fmt.Println("-- before")
		report(*lib, uuid)

		ask := func(network bool) fmt.Stringer {
			switch *kind {
			case "video":
				return video(uuid, network)
			case "live":
				return live(uuid, *size, network)
			}
			return request(uuid, *size, network)
		}
		fmt.Printf("-- local only:  %s\n", ask(false))
		fmt.Printf("-- network:     %s\n", ask(true))
		fmt.Printf("-- local again: %s\n", ask(false))

		time.Sleep(*wait)
		fmt.Printf("-- after (%s)\n", *wait)
		report(*lib, uuid)
	}
}

type result struct {
	seconds  float64
	w, h     int
	inCloud  bool
	degraded int
	progress float64
	err      string
}

func (r result) String() string {
	s := fmt.Sprintf("%.2fs %dx%d inCloud=%v degraded=%d", r.seconds, r.w, r.h, r.inCloud, r.degraded)
	if r.progress >= 0 {
		s += fmt.Sprintf(" progress=%.2f", r.progress)
	}
	if r.err != "" {
		s += " ERROR " + r.err
	}
	return s
}

func request(uuid string, size int, network bool) result {
	cu := C.CString(uuid)
	defer C.free(unsafe.Pointer(cu))
	n := 0
	if network {
		n = 1
	}
	r := C.pk_request(cu, C.int(size), C.int(n))
	out := result{float64(r.seconds), int(r.width), int(r.height), r.inCloud != 0, int(r.degraded), float64(r.progress), ""}
	if r.error != nil {
		out.err = C.GoString(r.error)
		C.free(unsafe.Pointer(r.error))
	}
	return out
}

func live(uuid string, size int, network bool) result {
	cu := C.CString(uuid)
	defer C.free(unsafe.Pointer(cu))
	n := 0
	if network {
		n = 1
	}
	r := C.pk_live(cu, C.int(size), C.int(n))
	out := result{float64(r.seconds), int(r.width), int(r.height), r.inCloud != 0, int(r.degraded), float64(r.progress), ""}
	if r.error != nil {
		out.err = C.GoString(r.error)
		C.free(unsafe.Pointer(r.error))
	}
	return out
}

type videoResult struct {
	seconds, duration float64
	w, h              int
	inCloud           bool
	url, err          string
}

func (r videoResult) String() string {
	s := fmt.Sprintf("%.2fs %dx%d %.1fs-long inCloud=%v", r.seconds, r.w, r.h, r.duration, r.inCloud)
	if r.url != "" {
		s += " url=" + r.url
	}
	if r.err != "" {
		s += " ERROR " + r.err
	}
	return s
}

func video(uuid string, network bool) videoResult {
	cu := C.CString(uuid)
	defer C.free(unsafe.Pointer(cu))
	n := 0
	if network {
		n = 1
	}
	r := C.pk_video(cu, C.int(n))
	out := videoResult{seconds: float64(r.seconds), duration: float64(r.duration), w: int(r.width), h: int(r.height), inCloud: r.inCloud != 0}
	if r.url != nil {
		out.url = C.GoString(r.url)
		C.free(unsafe.Pointer(r.url))
	}
	if r.error != nil {
		out.err = C.GoString(r.error)
		C.free(unsafe.Pointer(r.error))
	}
	return out
}

// report: what PhotoKit, the DB and the disk say about the asset's renditions
func report(lib, uuid string) {
	cu := C.CString(uuid)
	res := C.pk_resources(cu)
	C.free(unsafe.Pointer(cu))
	fmt.Print("   PhotoKit resources:\n" + indent(C.GoString(res)))
	C.free(unsafe.Pointer(res))

	fmt.Print("   DB (recipe: local/remote, size, bytes):\n" + indent(dbResources(lib, uuid)))

	x := strings.ToUpper(uuid[:1])
	var files []string
	for _, pattern := range []string{
		filepath.Join(lib, "resources", "derivatives", x, uuid+"*"),
		filepath.Join(lib, "resources", "derivatives", "masters", x, uuid+"*"),
		filepath.Join(lib, "resources", "derivatives", "cvt", x, uuid, "*"), // a video's frames
		filepath.Join(lib, "resources", "renders", x, uuid+"*"),
		filepath.Join(lib, "originals", x, uuid+"*"),
	} {
		m, _ := filepath.Glob(pattern)
		for _, f := range m {
			st, err := os.Stat(f)
			if err == nil {
				rel, _ := filepath.Rel(lib, f)
				files = append(files, fmt.Sprintf("%s %d", rel, st.Size()))
			}
		}
	}
	if len(files) == 0 {
		files = []string{"(none — or the library is not readable from here)"}
	}
	fmt.Print("   files:\n" + indent(strings.Join(files, "\n")+"\n"))
}

// dbResources: ZINTERNALRESOURCE rows of the asset, from a fresh copy of the DB
// (Photos keeps it open; the copy is what gontroller reads too)
func dbResources(lib, uuid string) string {
	tmp, err := os.MkdirTemp("", "photokit-spike")
	if err != nil {
		return err.Error() + "\n"
	}
	defer os.RemoveAll(tmp)
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := filepath.Join(lib, "database", "Photos.sqlite"+suffix)
		b, err := os.ReadFile(src)
		if err != nil {
			if suffix == "" {
				return "cannot read the DB: " + err.Error() + "\n"
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(tmp, "Photos.sqlite"+suffix), b, 0o600); err != nil {
			return err.Error() + "\n"
		}
	}
	q := `select r.ZRECIPEID || ': ' || r.ZLOCALAVAILABILITY || '/' || r.ZREMOTEAVAILABILITY || ', ' ||
	             r.ZUNORIENTEDWIDTH || 'x' || r.ZUNORIENTEDHEIGHT || ', ' || r.ZDATALENGTH
	      from ZINTERNALRESOURCE r join ZASSET a on a.Z_PK = r.ZASSET
	      where a.ZUUID = '` + strings.ReplaceAll(uuid, "'", "") + `' order by r.ZRECIPEID`
	out, err := exec.Command("sqlite3", "-readonly", filepath.Join(tmp, "Photos.sqlite"), q).CombinedOutput()
	if err != nil {
		return "sqlite3: " + err.Error() + " " + string(out)
	}
	return string(out)
}

func indent(s string) string {
	if s == "" {
		return ""
	}
	return "     " + strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", "\n     ") + "\n"
}
