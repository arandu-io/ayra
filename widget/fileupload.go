package widget

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// File is a file the person chose, as the application hands it to the upload.
//
// It carries a way to read the file and not its bytes: a file of a gigabyte is
// read when it is sent, a piece at a time, and never held.
type File struct {
	// Name is what the file is called, without a directory.
	Name string
	// Size is how many bytes it holds, and what the progress is a fraction of.
	// Zero is unknown, and the bar then has no fraction to show.
	Size int64
	// Open reads the file. It is called once per attempt: a retry opens it again.
	Open func() (io.ReadCloser, error)
}

// Chooser is how the person picks files, and it is the application's: a window
// has the platform's own dialog, a phone has its own picker, and which one is
// reachable is a fact about the platform that this library cannot decide. A
// [FilePickerProps] drawn on a screen of its own is one way to write it. It runs
// off the frame loop and may block on the dialog.
type Chooser interface {
	Choose(ctx context.Context) ([]File, error)
}

// ChooserFunc is a function as a [Chooser].
type ChooserFunc func(ctx context.Context) ([]File, error)

// Choose calls the function.
func (f ChooserFunc) Choose(ctx context.Context) ([]File, error) { return f(ctx) }

// Uploader sends one file to wherever the application keeps them. Body is the
// file, and the number of bytes read from it is the progress: an implementation
// that streams the body into a request reports exactly how far the upload is
// with no further call. The context is cancelled when the person cancels.
//
// Where it goes -- a signed URL, the application's own server -- and what
// authorizes it are the implementation's. This library reaches nothing by
// itself.
type Uploader interface {
	Upload(ctx context.Context, file File, body io.Reader) error
}

// UploaderFunc is a function as an [Uploader].
type UploaderFunc func(ctx context.Context, file File, body io.Reader) error

// Upload calls the function.
func (f UploaderFunc) Upload(ctx context.Context, file File, body io.Reader) error {
	return f(ctx, file, body)
}

// UploadStatus is where one file of an upload is.
type UploadStatus uint8

const (
	// Queued waits for a place.
	Queued UploadStatus = iota
	// Sending is on its way.
	Sending
	// Sent arrived.
	Sent
	// Failed did not, and the error says why.
	Failed
	// Cancelled was stopped by the person.
	Cancelled
	// Refused never started: it broke a rule of the field.
	Refused
)

// String names the status.
func (s UploadStatus) String() string {
	return [...]string{"queued", "sending", "sent", "failed", "cancelled", "refused"}[s]
}

// upload is one file and what has happened to it.
type upload struct {
	file   File
	status UploadStatus
	err    error
	sent   atomic.Int64
	cancel context.CancelFunc
	button Button
	remove Button
}

// FileUpload is the state of a field that chooses files and sends them: the
// list, and the work in flight. The caller holds one per field.
type FileUpload struct {
	mu      sync.Mutex
	items   []*upload
	choose  Button
	pending bool
	err     error
	changed bool
	last    atomic.Int64
}

// Add puts files in the list as if they were chosen, for a platform that hands
// them over by a gesture of its own -- a file dropped on the window -- and for a
// test.
func (u *FileUpload) Add(c ayra.Context, p FileUploadProps, files ...File) {
	u.mu.Lock()
	for _, file := range files {
		listed := 0
		for _, other := range u.items {
			if other.status != Refused {
				listed++
			}
		}
		item := &upload{file: file}
		if reason := p.refuse(file, listed); reason != nil {
			item.status, item.err = Refused, reason
		}
		u.items = append(u.items, item)
	}
	u.mu.Unlock()
	u.start(c, p)
}

// Sent is the files that arrived.
func (u *FileUpload) Sent() []File {
	u.mu.Lock()
	defer u.mu.Unlock()
	var sent []File
	for _, item := range u.items {
		if item.status == Sent {
			sent = append(sent, item.file)
		}
	}
	return sent
}

// Status is the status of the file at an index in the list, and the error that
// ended it if one did.
func (u *FileUpload) Status(index int) (UploadStatus, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if index < 0 || index >= len(u.items) {
		return Queued, nil
	}
	return u.items[index].status, u.items[index].err
}

// Progress is the fraction of a file sent, between 0 and 1, and zero when the
// size is not known.
func (u *FileUpload) Progress(index int) float32 {
	u.mu.Lock()
	defer u.mu.Unlock()
	if index < 0 || index >= len(u.items) || u.items[index].file.Size <= 0 {
		return 0
	}
	item := u.items[index]
	return min(float32(item.sent.Load())/float32(item.file.Size), 1)
}

// Changed reports that a file arrived, failed or was removed, once, and
// consumes it.
func (u *FileUpload) Changed() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	changed := u.changed
	u.changed = false
	return changed
}

// Busy reports whether any file is queued or on its way.
func (u *FileUpload) Busy() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, item := range u.items {
		if item.status == Queued || item.status == Sending {
			return true
		}
	}
	return false
}

// FileUploadProps is a field that chooses files and sends them, with a bar for
// each -- the counterpart of Kyse's FileUpload. Choosing and sending are two
// jobs and the application provides both: a [Chooser] that finds the files and
// an [Uploader] that takes them.
//
// The rules of the field -- what kinds, how large, how many -- are checked
// before anything is sent, and a file that breaks one is listed as refused with
// the reason, and never sent. They are a convenience for the person and not a
// control: the server that receives the file checks it again, because a client
// is not where a limit is enforced.
type FileUploadProps struct {
	// ComponentProps is what the caller changes on this one instance. The field
	// publishes "root", "zone" and "item"; the bar is a [ProgressProps] and is restyled
	// under the name "progress".
	ComponentProps
	// Chooser finds the files. Nil draws no choose button, for a field filled by
	// [FileUpload.Add] alone.
	Chooser Chooser
	// Uploader sends them. Nil lists the files and sends nothing.
	Uploader Uploader
	// Accept are the extensions allowed, with the dot: ".pdf", ".jpg". Empty
	// allows any.
	Accept []string
	// MaxSize is the largest file, in bytes. Zero is no limit.
	MaxSize int64
	// MaxFiles is how many files the list holds. Zero is no limit.
	MaxFiles int
	// Concurrency is how many files are sent at once. Zero takes two.
	Concurrency int
	// Words are the field's own text. Zero values draw English.
	Words FileUploadWords
	// Disabled draws it as unavailable and refuses to choose.
	Disabled bool
}

// FileUploadWords are the sentences a field says, in the application's language.
type FileUploadWords struct {
	Choose, Cancel, Retry, Remove, Empty string
	Sent, Queued                         string
	TooLarge, WrongKind, TooMany         string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p FileUploadProps) PartNames() []string {
	return []string{"root", "zone", "item"}
}

// refuse answers why a file may not be listed, or nil.
func (p FileUploadProps) refuse(file File, listed int) error {
	switch {
	case p.MaxFiles > 0 && listed >= p.MaxFiles:
		return errors.New(orDefault(p.Words.TooMany, "too many files"))
	case p.MaxSize > 0 && file.Size > p.MaxSize:
		return errors.New(orDefault(p.Words.TooLarge, "larger than allowed"))
	case len(p.Accept) > 0:
		extension := strings.ToLower(path.Ext(file.Name))
		for _, accepted := range p.Accept {
			if strings.EqualFold(accepted, extension) {
				return nil
			}
		}
		return errors.New(orDefault(p.Words.WrongKind, "this kind of file is not allowed"))
	}
	return nil
}

// start sends queued files, as many as the concurrency allows.
func (u *FileUpload) start(c ayra.Context, p FileUploadProps) {
	if p.Uploader == nil {
		return
	}
	limit := p.Concurrency
	if limit <= 0 {
		limit = 2
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	running := 0
	for _, item := range u.items {
		if item.status == Sending {
			running++
		}
	}
	for _, item := range u.items {
		if running >= limit {
			return
		}
		if item.status != Queued {
			continue
		}
		item.status, running = Sending, running+1
		ctx, cancel := context.WithCancel(context.Background())
		item.cancel = cancel
		go u.send(c, p, item, ctx)
	}
}

// send uploads one file and records how it ended.
func (u *FileUpload) send(c ayra.Context, p FileUploadProps, item *upload, ctx context.Context) {
	item.sent.Store(0)

	body, err := item.file.Open()
	if err == nil {
		err = p.Uploader.Upload(ctx, item.file, &counting{r: body, item: item, upload: u, redraw: c.Redraw})
		body.Close()
	}

	u.mu.Lock()
	switch {
	case ctx.Err() != nil:
		item.status, item.err = Cancelled, nil
	case err != nil:
		item.status, item.err = Failed, err
	default:
		item.status, item.err = Sent, nil
		if item.file.Size > 0 {
			item.sent.Store(item.file.Size)
		}
	}
	item.cancel = nil
	u.changed = true
	u.mu.Unlock()

	c.Redraw()
	u.start(c, p)
}

// counting is the body of an upload: it counts the bytes the uploader reads,
// which is the progress, and asks for a frame at most ten times a second so that
// a fast upload does not become a stream of redraws.
type counting struct {
	r      io.Reader
	item   *upload
	upload *FileUpload
	redraw func()
}

func (c *counting) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.item.sent.Add(int64(n))
	if now := time.Now().UnixMilli(); now-c.upload.last.Load() >= 100 {
		c.upload.last.Store(now)
		c.redraw()
	}
	return n, err
}

// Layout draws the field and returns the room it took.
func (p FileUploadProps) Layout(c ayra.Context, state *FileUpload) ayra.Dimensions {
	t := c.Theme

	if state.choose.Clicked(c) && p.Chooser != nil && !p.Disabled {
		state.mu.Lock()
		already := state.pending
		state.pending = true
		state.mu.Unlock()
		if !already {
			redraw := c.Redraw
			go func() {
				files, err := p.Chooser.Choose(context.Background())
				state.mu.Lock()
				state.pending, state.err = false, err
				state.mu.Unlock()
				if err == nil && len(files) > 0 {
					state.Add(c, p, files...)
				}
				redraw()
			}()
		}
	}

	state.mu.Lock()
	items := append([]*upload(nil), state.items...)
	chooseError := state.err
	state.mu.Unlock()

	for index := 0; index < len(items); index++ {
		item := items[index]
		switch {
		case item.button.Clicked(c):
			state.mu.Lock()
			switch item.status {
			case Sending:
				if item.cancel != nil {
					item.cancel()
				}
			case Failed, Cancelled:
				item.status, item.err = Queued, nil
			}
			state.mu.Unlock()
			state.start(c, p)
		case item.remove.Clicked(c):
			state.mu.Lock()
			for i, other := range state.items {
				if other == item {
					state.items = append(state.items[:i], state.items[i+1:]...)
					break
				}
			}
			state.changed = true
			state.mu.Unlock()
			items = append(items[:index], items[index+1:]...)
			index--
		}
	}

	root := t.Resolve("fileupload", "root", p.VariantName("default"), theme.Style{}, p.ComponentProps)
	zone := t.Resolve("fileupload", "zone", p.VariantName("default"), theme.Style{}, p.ComponentProps)

	return Surface(c, root, func(c ayra.Context) ayra.Dimensions {
		rows := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Surface(c.With(gtx), zone, func(c ayra.Context) ayra.Dimensions {
					c.Constraints.Min.X = c.Constraints.Max.X
					return layout.Inset{Top: 14, Bottom: 14}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
						inner := c.With(gtx)
						inner.Constraints.Min = inner.Constraints.Max
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							inner := c.With(gtx)
							inner.Constraints.Min = layout.Constraints{}.Min
							if p.Chooser == nil {
								return drawText(inner, orDefault(p.Words.Empty, "Drop files here"), unit.Sp(t.Type.Body), t.Colours.MutedForeground, 1, text.Middle, plain())
							}
							return ButtonProps{Label: orDefault(p.Words.Choose, "Choose files"), Variant: Outline, Disabled: p.Disabled}.Layout(inner, &state.choose)
						})
					})
				})
			}),
		}
		if chooseError != nil {
			rows = append(rows, layout.Rigid(layout.Spacer{Height: 8}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return AlertProps{Title: chooseError.Error(), Severity: Failure}.Layout(c.With(gtx))
			}))
		}
		for _, item := range items {
			item := item
			rows = append(rows, layout.Rigid(layout.Spacer{Height: 8}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.item(c.With(gtx), item)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, rows...)
	})
}

// item draws one file: its name, where it is, and what can be done to it.
func (p FileUploadProps) item(c ayra.Context, item *upload) ayra.Dimensions {
	t := c.Theme

	state, err, sent := item.status, item.err, item.sent.Load()
	style := t.Resolve("fileupload", "item", p.VariantName(state.String()), theme.Style{}, p.ComponentProps)

	status := p.statusText(item, state, sent)
	ink := t.Colours.MutedForeground
	if state == Failed || state == Refused {
		ink = t.Colours.Destructive
		if err != nil {
			status = err.Error()
		}
	}

	action, handled := "", false
	switch state {
	case Sending:
		action, handled = orDefault(p.Words.Cancel, "Cancel"), true
	case Failed, Cancelled:
		action, handled = orDefault(p.Words.Retry, "Retry"), true
	}

	return Surface(c, style, func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return drawText(c.With(gtx), item.file.Name, unit.Sp(t.Type.Body), t.Colours.Foreground, 1, text.Start, plain())
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						inner := c.With(gtx)
						inner.Constraints.Min = layout.Constraints{}.Min
						return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return drawText(inner.With(gtx), status, unit.Sp(t.Type.Small), ink, 1, text.End, plain())
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						inner := c.With(gtx)
						inner.Constraints.Min = layout.Constraints{}.Min
						if !handled {
							return layout.Dimensions{}
						}
						return ButtonProps{Label: action, Variant: Ghost, Size: ExtraSmall}.Layout(inner, &item.button)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						inner := c.With(gtx)
						inner.Constraints.Min = layout.Constraints{}.Min
						if state == Sending {
							return layout.Dimensions{}
						}
						return ButtonProps{Label: orDefault(p.Words.Remove, "Remove"), Variant: Ghost, Size: ExtraSmall}.Layout(inner, &item.remove)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if state != Sending && state != Queued {
					return layout.Dimensions{}
				}
				return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					fraction := float32(0)
					if item.file.Size > 0 {
						fraction = min(float32(sent)/float32(item.file.Size), 1)
					}
					return ProgressProps{
						ComponentProps: ComponentProps{Parts: p.Parts, Custom: p.Custom},
						Fraction:       fraction, Indeterminate: item.file.Size <= 0,
					}.Layout(c.With(gtx))
				})
			}),
		)
	})
}

// statusText is the short line beside a file: how far it is, or where it ended.
func (p FileUploadProps) statusText(item *upload, state UploadStatus, sent int64) string {
	switch state {
	case Queued:
		return orDefault(p.Words.Queued, "Waiting")
	case Sending:
		if item.file.Size > 0 {
			return fmt.Sprintf("%d%%", sent*100/item.file.Size)
		}
		return byteSize(sent)
	case Sent:
		return orDefault(p.Words.Sent, "Sent")
	case Cancelled:
		return "-"
	}
	return byteSize(item.file.Size)
}

// byteSize writes a number of bytes the way a person reads it.
func byteSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
