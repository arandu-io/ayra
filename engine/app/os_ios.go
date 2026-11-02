package app

/*
#cgo CFLAGS: -DGLES_SILENCE_DEPRECATION -Werror -Wno-deprecated-declarations -fmodules -fobjc-arc -x objective-c

#include <CoreGraphics/CoreGraphics.h>
#include <UIKit/UIKit.h>
#include <stdint.h>
#include "framework_ios.h"

__attribute__ ((visibility ("hidden"))) int gio_applicationMain(int argc, char *argv[]);
__attribute__ ((visibility ("hidden"))) void gio_viewSetHandle(CFTypeRef viewRef, uintptr_t handle);

struct drawParams {
	CGFloat dpi, sdpi;
	CGFloat width, height;
	CGFloat top, right, bottom, left;
};

static void writeClipboard(unichar *chars, NSUInteger length) {
#if !TARGET_OS_TV
	@autoreleasepool {
		NSString *s = [NSString string];
		if (length > 0) {
			s = [NSString stringWithCharacters:chars length:length];
		}
		UIPasteboard *p = UIPasteboard.generalPasteboard;
		p.string = s;
	}
#endif
}

static CFTypeRef readClipboard(void) {
#if !TARGET_OS_TV
	@autoreleasepool {
		UIPasteboard *p = UIPasteboard.generalPasteboard;
		return (__bridge_retained CFTypeRef)p.string;
	}
#else
	return nil;
#endif
}

// setBars gives the screen's ground its colour and says whether the status bar
// is drawn over a light one, which is what picks dark or light content for it.
// The controller's view is what shows in the strips of the safe area.
static void setBars(CFTypeRef controllerRef, CGFloat r, CGFloat g, CGFloat b, CGFloat a, int light) {
#if !TARGET_OS_TV
	AyraViewController *vc = (__bridge AyraViewController *)controllerRef;
	dispatch_async(dispatch_get_main_queue(), ^{
		vc.barsLight = light != 0;
		vc.view.backgroundColor = [UIColor colorWithRed:r green:g blue:b alpha:a];
		[vc setNeedsStatusBarAppearanceUpdate];
	});
#endif
}

static void showTextInput(CFTypeRef viewRef) {
	UIView *view = (__bridge UIView *)viewRef;
	[view becomeFirstResponder];
}

static void hideTextInput(CFTypeRef viewRef) {
	UIView *view = (__bridge UIView *)viewRef;
	[view resignFirstResponder];
}

// viewAppearance is the interface style the view is drawn under: 1 for light,
// 2 for dark, 0 when the system does not say. The trait collection of a view
// follows the window and, through it, the system setting, which is why it is
// read here and not from the screen.
static int viewAppearance(CFTypeRef viewRef) {
	UIView *v = (__bridge UIView *)viewRef;
	if (@available(iOS 12.0, tvOS 10.0, *)) {
		switch (v.traitCollection.userInterfaceStyle) {
		case UIUserInterfaceStyleLight:
			return 1;
		case UIUserInterfaceStyleDark:
			return 2;
		default:
			return 0;
		}
	}
	return 0;
}

static struct drawParams viewDrawParams(CFTypeRef viewRef) {
	UIView *v = (__bridge UIView *)viewRef;
	struct drawParams params;
	CGFloat scale = v.layer.contentsScale;
	// Use 163 as the standard ppi on iOS.
	params.dpi = 163*scale;
	params.sdpi = params.dpi;
	UIEdgeInsets insets = v.layoutMargins;
	if (@available(iOS 11.0, tvOS 11.0, *)) {
		UIFontMetrics *metrics = [UIFontMetrics defaultMetrics];
		params.sdpi = [metrics scaledValueForValue:params.sdpi];
		insets = v.safeAreaInsets;
	}
	params.width = v.bounds.size.width*scale;
	params.height = v.bounds.size.height*scale;
	params.top = insets.top*scale;
	params.right = insets.right*scale;
	params.bottom = insets.bottom*scale;
	params.left = insets.left*scale;
	return params;
}
*/
import "C"

import (
	"image"
	"io"
	"os"
	"runtime"
	"runtime/cgo"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/io/transfer"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/unit"
)

type UIKitViewEvent struct {
	// ViewController is a CFTypeRef for the UIViewController backing a Window.
	ViewController uintptr
}

type window struct {
	view        C.CFTypeRef
	controller  C.CFTypeRef
	w           *callbacks
	displayLink *displayLink
	loop        *eventLoop

	hidden bool
	cursor pointer.Cursor
	config Config

	pointerMap []C.CFTypeRef
}

var mainWindow = newWindowRendezvous()

func init() {
	// Darwin requires UI operations happen on the main thread only.
	runtime.LockOSThread()
}

//export onCreate
func onCreate(view, controller C.CFTypeRef) {
	wopts := <-mainWindow.out
	w := &window{
		view:       view,
		controller: controller,
		w:          wopts.window,
	}
	w.loop = newEventLoop(w.w, w.wakeup)
	w.w.SetDriver(w)
	mainWindow.windows <- struct{}{}
	dl, err := newDisplayLink(func() {
		w.draw(false)
	})
	if err != nil {
		w.w.ProcessEvent(DestroyEvent{Err: err})
		return
	}
	w.displayLink = dl
	C.gio_viewSetHandle(view, C.uintptr_t(cgo.NewHandle(w)))
	w.Configure(wopts.options)
	w.ProcessEvent(UIKitViewEvent{ViewController: uintptr(controller)})
}

func viewFor(h C.uintptr_t) *window {
	return cgo.Handle(h).Value().(*window)
}

//export gio_onDraw
func gio_onDraw(h C.uintptr_t) {
	w := viewFor(h)
	w.draw(true)
}

func (w *window) draw(sync bool) {
	if w.hidden {
		return
	}
	if appearance := appearanceFromCode(int(C.viewAppearance(w.view))); appearance != w.config.Appearance {
		w.config.Appearance = appearance
		w.ProcessEvent(ConfigEvent{Config: w.config})
	}
	params := C.viewDrawParams(w.view)
	if params.width == 0 || params.height == 0 {
		return
	}
	const inchPrDp = 1.0 / 163
	m := unit.Metric{
		PxPerDp: float32(params.dpi) * inchPrDp,
		PxPerSp: float32(params.sdpi) * inchPrDp,
	}
	dppp := unit.Dp(1. / m.PxPerDp)
	w.ProcessEvent(frameEvent{
		FrameEvent: FrameEvent{
			Now: time.Now(),
			Size: image.Point{
				X: int(params.width + .5),
				Y: int(params.height + .5),
			},
			Insets: Insets{
				Top:    unit.Dp(params.top) * dppp,
				Bottom: unit.Dp(params.bottom) * dppp,
				Left:   unit.Dp(params.left) * dppp,
				Right:  unit.Dp(params.right) * dppp,
			},
			Metric: m,
		},
		Sync: sync,
	})
}

//export onStop
func onStop(h C.uintptr_t) {
	w := viewFor(h)
	w.hidden = true
	w.ProcessEvent(StageEvent{Stage: StageBackground})
}

//export onStart
func onStart(h C.uintptr_t) {
	w := viewFor(h)
	w.hidden = false
	w.ProcessEvent(StageEvent{Stage: StageForeground})
	w.draw(true)
}

//export onDestroy
func onDestroy(h C.uintptr_t) {
	w := viewFor(h)
	w.ProcessEvent(UIKitViewEvent{})
	w.ProcessEvent(DestroyEvent{})
	w.displayLink.Close()
	w.displayLink = nil
	cgo.Handle(h).Delete()
	w.view = 0
}

//export onFocus
func onFocus(h C.uintptr_t, focus int) {
	w := viewFor(h)
	w.config.Focused = focus != 0
	w.ProcessEvent(ConfigEvent{Config: w.config})
}

//export onLowMemory
func onLowMemory() {
	runtime.GC()
	debug.FreeOSMemory()
}

//export onUpArrow
func onUpArrow(h C.uintptr_t) {
	viewFor(h).onKeyCommand(key.NameUpArrow)
}

//export onDownArrow
func onDownArrow(h C.uintptr_t) {
	viewFor(h).onKeyCommand(key.NameDownArrow)
}

//export onLeftArrow
func onLeftArrow(h C.uintptr_t) {
	viewFor(h).onKeyCommand(key.NameLeftArrow)
}

//export onRightArrow
func onRightArrow(h C.uintptr_t) {
	viewFor(h).onKeyCommand(key.NameRightArrow)
}

//export onDeleteBackward
func onDeleteBackward(h C.uintptr_t) {
	viewFor(h).onKeyCommand(key.NameDeleteBackward)
}

//export onText
func onText(h C.uintptr_t, str C.CFTypeRef) {
	w := viewFor(h)
	w.w.EditorInsert(nsstringToString(str))
}

//export onTouch
func onTouch(h C.uintptr_t, last C.int, touchRef C.CFTypeRef, phase C.NSInteger, x, y C.CGFloat, ti C.double) {
	var kind pointer.Kind
	switch phase {
	case C.UITouchPhaseBegan:
		kind = pointer.Press
	case C.UITouchPhaseMoved:
		kind = pointer.Move
	case C.UITouchPhaseEnded:
		kind = pointer.Release
	case C.UITouchPhaseCancelled:
		kind = pointer.Cancel
	default:
		return
	}
	w := viewFor(h)
	t := time.Duration(float64(ti) * float64(time.Second))
	p := f32.Point{X: float32(x), Y: float32(y)}
	w.ProcessEvent(pointer.Event{
		Kind:      kind,
		Source:    pointer.Touch,
		PointerID: w.lookupTouch(last != 0, touchRef),
		Position:  p,
		Time:      t,
	})
}

func (w *window) ReadClipboard() {
	cstr := C.readClipboard()
	defer C.CFRelease(cstr)
	content := nsstringToString(cstr)
	w.ProcessEvent(transfer.DataEvent{
		Type: "application/text",
		Open: func() io.ReadCloser {
			return io.NopCloser(strings.NewReader(content))
		},
	})
}

func (w *window) WriteClipboard(mime string, s []byte) {
	u16 := utf16.Encode([]rune(string(s)))
	chars := (*C.unichar)(unsafe.Pointer(unsafe.SliceData(u16)))
	C.writeClipboard(chars, C.NSUInteger(len(u16)))
}

func (w *window) Configure(options []Option) {
	cnf := w.config
	cnf.apply(unit.Metric{}, options)
	// The status bar follows the colour the application asked for: dark content
	// over a light ground and light content over a dark one, each time it changes.
	if cnf.StatusColor != w.config.StatusColor && w.controller != 0 {
		c := cnf.StatusColor
		light := 0.299*float32(c.R)+0.587*float32(c.G)+0.114*float32(c.B) > 127.5
		lit := C.int(0)
		if light {
			lit = 1
		}
		C.setBars(w.controller, C.CGFloat(c.R)/255, C.CGFloat(c.G)/255, C.CGFloat(c.B)/255, C.CGFloat(c.A)/255, lit)
	}
	w.config = cnf
	// Decorations are never disabled.
	w.config.Decorated = true
	w.ProcessEvent(ConfigEvent{Config: w.config})
}

func (w *window) EditorStateChanged(old, new editorState) {}

func (w *window) Perform(system.Action) {}

func (w *window) SetAnimating(anim bool) {
	if anim {
		w.displayLink.Start()
	} else {
		w.displayLink.Stop()
	}
}

func (w *window) SetCursor(cursor pointer.Cursor) {
	w.cursor = windowSetCursor(w.cursor, cursor)
}

func (w *window) onKeyCommand(name key.Name) {
	w.ProcessEvent(key.Event{
		Name: name,
	})
}

// lookupTouch maps an UITouch pointer value to an index. If
// last is set, the map is cleared.
func (w *window) lookupTouch(last bool, touch C.CFTypeRef) pointer.ID {
	id := -1
	for i, ref := range w.pointerMap {
		if ref == touch {
			id = i
			break
		}
	}
	if id == -1 {
		id = len(w.pointerMap)
		w.pointerMap = append(w.pointerMap, touch)
	}
	if last {
		w.pointerMap = w.pointerMap[:0]
	}
	return pointer.ID(id)
}

func (w *window) contextView() C.CFTypeRef {
	return w.view
}

func (w *window) ShowTextInput(show bool) {
	if show {
		C.showTextInput(w.view)
	} else {
		C.hideTextInput(w.view)
	}
}

func (w *window) SetInputHint(_ key.InputHint) {}

func (w *window) ProcessEvent(e event.Event) {
	w.w.ProcessEvent(e)
	w.loop.FlushEvents()
}

func (w *window) Event() event.Event {
	return w.loop.Event()
}

func (w *window) Invalidate() {
	w.loop.Invalidate()
}

func (w *window) Run(f func()) {
	w.loop.Run(f)
}

func (w *window) Frame(frame *op.Ops) {
	w.loop.Frame(frame)
}

func newWindow(win *callbacks, options []Option) {
	mainWindow.in <- windowAndConfig{win, options}
	<-mainWindow.windows
}

var mainMode = mainModeUndefined

const (
	mainModeUndefined = iota
	mainModeExe
	mainModeLibrary
)

func osMain() {
	switch mainMode {
	case mainModeUndefined:
		if !isMainThread() {
			panic("app.Main must be run on the main goroutine")
		}

		mainMode = mainModeExe
		var argv []*C.char
		for _, arg := range os.Args {
			a := C.CString(arg)
			defer C.free(unsafe.Pointer(a))
			argv = append(argv, a)
		}
		C.gio_applicationMain(C.int(len(argv)), unsafe.SliceData(argv))
	case mainModeExe:
		panic("app.Main may be called only once")
	case mainModeLibrary:
		// Do nothing, we're embedded as a library.
	}
	select {}
}

//export gio_onOpenURI
func gio_onOpenURI(uri C.CFTypeRef) {
	evt, err := newURLEvent(nsstringToString(uri))
	if err != nil {
		return
	}
	processGlobalEvent(evt)
}

//export gio_runMain
func gio_runMain() {
	if !isMainThread() {
		panic("app.Main must be run on the main goroutine")
	}
	switch mainMode {
	case mainModeUndefined:
		mainMode = mainModeLibrary
		runMain()
	case mainModeExe:
		// Do nothing, main has already been called.
	}
}

func (UIKitViewEvent) implementsViewEvent() {}
func (UIKitViewEvent) ImplementsEvent()     {}
func (u UIKitViewEvent) Valid() bool {
	return u != (UIKitViewEvent{})
}
