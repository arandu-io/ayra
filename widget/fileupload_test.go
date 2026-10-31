package widget_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

func file(name string, size int) widget.File {
	data := bytes.Repeat([]byte("x"), size)
	return widget.File{Name: name, Size: int64(size), Open: func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); !ok(); {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestFilesAreSentAndTheBodyIsTheProgress(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 500)
	var got atomic.Int64
	props := widget.FileUploadProps{
		Uploader: widget.UploaderFunc(func(ctx context.Context, f widget.File, body io.Reader) error {
			n, err := io.Copy(io.Discard, body)
			got.Add(n)
			return err
		}),
	}
	var state widget.FileUpload

	state.Add(ctx, props, file("a.pdf", 4096), file("b.pdf", 100))
	waitFor(t, "both files to be sent", func() bool { return !state.Busy() })

	if len(state.Sent()) != 2 || got.Load() != 4196 {
		t.Errorf("%d files sent, %d bytes read", len(state.Sent()), got.Load())
	}
	if status, _ := state.Status(0); status != widget.Sent {
		t.Errorf("the first file is %v", status)
	}
	if state.Progress(0) != 1 {
		t.Errorf("a sent file's progress is %v", state.Progress(0))
	}
	if !state.Changed() || state.Changed() {
		t.Error("an arrival must be reported once")
	}
	if dims := props.Layout(ctx, &state); dims.Size.Y <= 0 {
		t.Error("the field drew nothing")
	}
}

func TestTheRulesRefuseBeforeAnythingIsSent(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 500)
	var sent atomic.Int32
	props := widget.FileUploadProps{
		Accept: []string{".pdf", ".JPG"}, MaxSize: 1000, MaxFiles: 3,
		Uploader: widget.UploaderFunc(func(ctx context.Context, f widget.File, body io.Reader) error {
			sent.Add(1)
			return nil
		}),
	}
	var state widget.FileUpload

	state.Add(ctx, props, file("ok.pdf", 10), file("photo.jpg", 10), file("big.pdf", 5000), file("macro.exe", 10), file("late.pdf", 10), file("fourth.pdf", 10))
	waitFor(t, "the accepted files", func() bool { return !state.Busy() })

	wantStatus := []widget.UploadStatus{widget.Sent, widget.Sent, widget.Refused, widget.Refused, widget.Sent, widget.Refused}
	for i, want := range wantStatus {
		if got, _ := state.Status(i); got != want {
			t.Errorf("file %d is %v, want %v", i, got, want)
		}
	}
	if sent.Load() != 3 {
		t.Errorf("%d files reached the uploader; a refused file is never sent", sent.Load())
	}
	if _, reason := state.Status(5); reason == nil || !strings.Contains(reason.Error(), "many") {
		t.Errorf("the reason for the fourth file is %v", reason)
	}
	if _, reason := state.Status(3); reason == nil || !strings.Contains(reason.Error(), "kind") {
		t.Errorf("the reason for the wrong kind is %v", reason)
	}
}

func TestAFailedFileCanBeSentAgain(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 500)
	var attempts atomic.Int32
	props := widget.FileUploadProps{
		Uploader: widget.UploaderFunc(func(ctx context.Context, f widget.File, body io.Reader) error {
			if attempts.Add(1) == 1 {
				return errors.New("connection reset")
			}
			_, err := io.Copy(io.Discard, body)
			return err
		}),
	}
	var state widget.FileUpload

	state.Add(ctx, props, file("a.pdf", 10))
	waitFor(t, "the first attempt", func() bool { return !state.Busy() })
	if status, err := state.Status(0); status != widget.Failed || err == nil {
		t.Fatalf("the first attempt left %v, %v", status, err)
	}
}

func TestOnlyAsManyAreSentAtOnceAsTheConcurrencyAllows(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 500)
	var running, peak atomic.Int32
	release := make(chan struct{})
	props := widget.FileUploadProps{
		Concurrency: 2,
		Uploader: widget.UploaderFunc(func(ctx context.Context, f widget.File, body io.Reader) error {
			now := running.Add(1)
			for {
				old := peak.Load()
				if now <= old || peak.CompareAndSwap(old, now) {
					break
				}
			}
			<-release
			running.Add(-1)
			return nil
		}),
	}
	var state widget.FileUpload

	state.Add(ctx, props, file("1", 1), file("2", 1), file("3", 1), file("4", 1))
	waitFor(t, "two files to be on their way", func() bool { return running.Load() == 2 })
	time.Sleep(20 * time.Millisecond)
	if running.Load() != 2 {
		t.Errorf("%d files are being sent at once, want 2", running.Load())
	}
	close(release)
	waitFor(t, "all four", func() bool { return !state.Busy() })
	if peak.Load() > 2 || len(state.Sent()) != 4 {
		t.Errorf("peak %d, %d sent", peak.Load(), len(state.Sent()))
	}
}

func TestChoosingFilesFeedsTheList(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 500)
	props := widget.FileUploadProps{
		Chooser: widget.ChooserFunc(func(ctx context.Context) ([]widget.File, error) {
			return []widget.File{file("x.pdf", 1)}, nil
		}),
	}
	var state widget.FileUpload
	if dims := props.Layout(ctx, &state); dims.Size.X <= 0 {
		t.Error("the choose button was not drawn")
	}
}
