package render

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"perceptrail/gontroller/internal/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

// ffmpeg: the video tools this server has — found at start, nil without them
type ffmpeg struct {
	bin, probe string
	tonemap    string // the filters mapping HDR to SDR (%s: the input's transfer, primaries, matrix); "": no zscale
}

// recipe: how an accelerator decodes, scales and encodes. The software one only for
// now; the hardware ones (QSV, VAAPI, NVENC, VideoToolbox) are rows to come.
type recipe struct {
	input  []string // before -i: the hardware decoder
	encode []string // the encoder and its quality
}

// stream: what ffprobe says of a video
type stream struct {
	w, h                        int
	duration                    time.Duration
	transfer, primaries, matrix string // its colour, as tagged ("" untagged)
}

var software = recipe{encode: []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
	"-pix_fmt", "yuv420p", "-profile:v", "high"}}

const (
	// hdrToSDR: the tone mapping, when ffmpeg has zscale (zimg); the input's colour
	// given, not guessed (zscale finds "no path" from an untagged one)
	hdrToSDR = "zscale=tin=%s:pin=%s:min=%s:t=linear:npl=100,format=gbrpf32le,zscale=p=bt709," +
		"tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p"
	// sdrTags: what the output is, on its frames — the encoder takes the colour tags
	// from the frames, not from -color_trc
	sdrTags = "setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709"
)

// findFFmpeg: ffmpeg and ffprobe that run, and whether this ffmpeg can tone map
func findFFmpeg(ctx context.Context, bin, probe string) (*ffmpeg, error) {
	if err := exec.CommandContext(ctx, probe, "-version").Run(); err != nil {
		return nil, fmt.Errorf("%s: %w", probe, err)
	}
	filters, err := exec.CommandContext(ctx, bin, "-hide_banner", "-filters").Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", bin, err)
	}
	f := &ffmpeg{bin: bin, probe: probe}
	if strings.Contains(string(filters), " zscale ") {
		f.tonemap = hdrToSDR
	}
	return f, nil
}

// video: a video's rendition — the long side at most size, never upscaled, H.264 and
// AAC that every browser plays — and, when poster says so, its poster's stills
// (a frame, through still). For a Live Photo's motion: no poster (its photo is).
//
// Not obvious:
//   - HDR (an iPhone's HLG, Dolby Vision) is tone mapped to SDR when this ffmpeg can
//     (zscale); without, it is kept as it is — HLG is made to look acceptable on SDR.
//   - Its time is its own: a long video gets longer than a photo (three times its
//     length, at least two minutes).
func (f *ffmpeg) video(ctx context.Context, guid api.GUID, src string, size int, dir, rel string,
	poster bool, still func(ctx context.Context, src string) ([]dto.RenditionDto, error)) ([]dto.RenditionDto, error) {
	in, err := f.info(ctx, src)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, max(2*time.Minute, 3*in.duration))
	defer cancel()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	filters := f.filters(in, size)
	name := fmt.Sprintf("%d.mp4", size)
	dst := filepath.Join(dir, name)
	args := append([]string{"-hide_banner", "-loglevel", "error", "-y"}, software.input...)
	args = append(args, "-i", src, "-map", "0:v:0", "-map", "0:a:0?", "-vf", filters)
	args = append(args, software.encode...)
	args = append(args, "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", dst)
	if msg, err := exec.CommandContext(ctx, f.bin, args...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, msg)
	}
	made, err := f.info(ctx, dst)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(dst)
	if err != nil {
		return nil, err
	}
	out := []dto.RenditionDto{{GUID: guid, Size: size, Format: "mp4", Role: dto.RoleMotion,
		W: made.w, H: made.h, Bytes: info.Size(), Path: filepath.Join(rel, name)}}
	if !poster {
		return out, nil
	}
	frame := filepath.Join(dir, "poster.png")
	defer os.Remove(frame)
	at := min(time.Second, in.duration/2)
	grab := []string{"-hide_banner", "-loglevel", "error", "-y", "-ss", strconv.FormatFloat(at.Seconds(), 'f', 3, 64),
		"-i", src, "-frames:v", "1"}
	if sdr := f.toSDR(in); sdr != "" {
		grab = append(grab, "-vf", sdr)
	}
	if msg, err := exec.CommandContext(ctx, f.bin, append(grab, frame)...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg poster: %w: %s", err, msg)
	}
	stills, err := still(ctx, frame)
	if err != nil {
		return nil, err
	}
	return append(stills, out...), nil
}

// filters: tone mapping when it is HDR and this ffmpeg can, then the size — shrunk to
// the long side, or only made even (H.264 wants even sides) — and the SDR tags
func (f *ffmpeg) filters(in stream, size int) string {
	var chain []string
	if sdr := f.toSDR(in); sdr != "" {
		chain = append(chain, sdr)
	}
	if max(in.w, in.h) > size {
		chain = append(chain, fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2", size, size))
	} else {
		chain = append(chain, "scale=trunc(iw/2)*2:trunc(ih/2)*2")
	}
	return strings.Join(append(chain, sdrTags), ",")
}

// toSDR: the tone mapping of an HDR input (PQ, HLG), when this ffmpeg has it; ""
// for SDR, or without zscale (HLG is made to look acceptable as it is)
func (f *ffmpeg) toSDR(in stream) string {
	if f.tonemap == "" || in.transfer != "smpte2084" && in.transfer != "arib-std-b67" {
		return ""
	}
	return fmt.Sprintf(f.tonemap, in.transfer, cmp.Or(in.primaries, "bt2020"), cmp.Or(in.matrix, "bt2020nc"))
}

// info: a video's first video stream as ffprobe sees it
func (f *ffmpeg) info(ctx context.Context, src string) (stream, error) {
	out, err := exec.CommandContext(ctx, f.probe, "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height,color_transfer,color_primaries,color_space:format=duration", "-of", "json", src).Output()
	if err != nil {
		return stream{}, fmt.Errorf("ffprobe: %w", err)
	}
	var probed struct {
		Streams []struct {
			Width, Height int
			Transfer      string `json:"color_transfer"`
			Primaries     string `json:"color_primaries"`
			Matrix        string `json:"color_space"`
		}
		Format struct {
			Duration string
		}
	}
	if err := json.Unmarshal(out, &probed); err != nil || len(probed.Streams) == 0 {
		return stream{}, fmt.Errorf("ffprobe: no video stream in %s", src)
	}
	seconds, _ := strconv.ParseFloat(probed.Format.Duration, 64)
	s := probed.Streams[0]
	untagged := func(v string) string { // ffprobe's word for none
		if v == "unknown" {
			return ""
		}
		return v
	}
	return stream{w: s.Width, h: s.Height, duration: time.Duration(seconds * float64(time.Second)),
		transfer: untagged(s.Transfer), primaries: untagged(s.Primaries), matrix: untagged(s.Matrix)}, nil
}
