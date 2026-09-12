package transfer_test

import (
	"image"
	"io"
	"strings"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/io/transfer"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
)

// The tests drive a whole gesture rather than comparing the declarations by
// hand, because a filter is only a claim: what decides whether two halves meet
// is the system reading both of them, and a test that asserted the claim would
// pass on a day when nothing was routed at all.

var (
	// sourceArea is where the drag starts, and targetArea is where it lands.
	// They are apart so that a press in one and a release in the other is an
	// unambiguous drop rather than a click.
	sourceArea = image.Rect(0, 0, 20, 20)
	targetArea = image.Rect(40, 0, 60, 20)

	inSource = f32.Pt(10, 10)
	inTarget = f32.Pt(50, 10)
)

// offer is data a source hands over, and it remembers being closed.
//
// Remembering is the point: the question these tests ask is not what the bytes
// were but who let go of them, and a reader that did not record its own close
// could answer neither half of that.
type offer struct {
	*strings.Reader
	closed bool
}

func newOffer(content string) *offer {
	return &offer{Reader: strings.NewReader(content)}
}

func (o *offer) Close() error {
	o.closed = true
	return nil
}

// scene is a source, its candidate targets, and the frame they live in.
type scene struct {
	router  input.Router
	ops     op.Ops
	source  event.Tag
	targets []event.Tag
}

// stage declares one source offering srcType and one target per entry of
// tgtTypes, then hands back the scene with its first frame already submitted.
//
// Every target is put in the same area, and put there passing events through,
// so that a single drop reaches all of them at once. The only thing left that
// can decide which one receives the data is then the type -- which is what
// these tests are about. Targets in separate areas would let a passing test be
// explained by the pointer having landed somewhere else, and targets stacked
// without passing through would let it be explained by the topmost one
// swallowing the drop.
func stage(srcType string, tgtTypes ...string) *scene {
	s := &scene{source: new(int)}

	drain(&s.router, -1, transfer.SourceFilter{Target: s.source, Type: srcType})
	area := clip.Rect(sourceArea).Push(&s.ops)
	event.Op(&s.ops, s.source)
	area.Pop()

	for _, tgtType := range tgtTypes {
		tgt := event.Tag(new(int))
		s.targets = append(s.targets, tgt)

		drain(&s.router, -1, transfer.TargetFilter{Target: tgt, Type: tgtType})
		area := clip.Rect(targetArea).Push(&s.ops)
		pass := pointer.PassOp{}.Push(&s.ops)
		event.Op(&s.ops, tgt)
		pass.Pop()
		area.Pop()
	}

	s.router.Frame(&s.ops)
	return s
}

// drag presses inside the source and moves, which is what starts a transfer.
func (s *scene) drag() {
	s.router.Queue(
		pointer.Event{Position: inSource, Kind: pointer.Press},
		pointer.Event{Position: inSource, Kind: pointer.Move},
	)
}

// drop releases inside the target area.
func (s *scene) drop() {
	s.router.Queue(pointer.Event{Position: inTarget, Kind: pointer.Release})
}

// sourceEvents reads what the source is told about a transfer of srcType.
func (s *scene) sourceEvents(n int, srcType string) []event.Event {
	return drain(&s.router, n, transfer.SourceFilter{Target: s.source, Type: srcType})
}

// targetEvents reads what the nth target is told about a transfer of tgtType.
func (s *scene) targetEvents(n int, target int, tgtType string) []event.Event {
	return drain(&s.router, n, transfer.TargetFilter{Target: s.targets[target], Type: tgtType})
}

// drain reads up to n events matching the filters, or every one of them for
// n of -1. Declaring the filters is what reading them costs, so this is also
// how a handler says it is still listening.
func drain(r *input.Router, n int, filters ...event.Filter) []event.Event {
	var collected []event.Event
	for n == -1 || len(collected) < n {
		e, ok := r.Event(filters...)
		if !ok {
			break
		}
		collected = append(collected, e)
	}
	return collected
}

// assertEvents compares a sequence against what was expected.
//
// Every event compared here has to be comparable, which rules out [DataEvent]:
// it carries a function, and two of those are never equal. A data event is
// therefore read apart by hand, which is also the only way to ask the question
// worth asking about it -- whether what arrived is what was offered.
func assertEvents(t *testing.T, got []event.Event, want ...event.Event) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d events %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("event %d is %#v, want %#v", i, got[i], want[i])
		}
	}
}

// TestOfferedTypeReachesOnlyTheTargetThatAsksForIt is the whole protocol in one
// gesture: two targets are under the drop, and the type is what tells them
// apart.
func TestOfferedTypeReachesOnlyTheTargetThatAsksForIt(t *testing.T) {
	const offered = "application/pdf"
	const unrelated = "text/plain"

	s := stage(offered, offered, unrelated)

	s.drag()
	assertEvents(t, s.targetEvents(1, 0, offered), transfer.InitiateEvent{})
	assertEvents(t, s.targetEvents(-1, 1, unrelated))

	s.drop()
	assertEvents(t, s.sourceEvents(1, offered), transfer.RequestEvent{Type: offered})

	s.router.Source().Execute(transfer.OfferCmd{Tag: s.source, Type: offered, Data: newOffer("x")})
	assertEvents(t, s.sourceEvents(-1, offered), transfer.CancelEvent{})

	accepted := s.targetEvents(-1, 0, offered)
	if len(accepted) != 2 {
		t.Fatalf("the matching target got %d events %v, want a data event and a cancel", len(accepted), accepted)
	}
	if _, ok := accepted[0].(transfer.DataEvent); !ok {
		t.Errorf("the matching target got %T first, want %T", accepted[0], transfer.DataEvent{})
	}
	assertEvents(t, accepted[1:], transfer.CancelEvent{})

	// The other target was under the same drop from beginning to end and heard
	// nothing at all -- not the initiation, not the data, not even the cancel.
	// A target is told about a transfer it could not have accepted only if the
	// types stopped deciding.
	assertEvents(t, s.targetEvents(-1, 1, unrelated))
}

// TestTypesMeetWholeOrNotAtAll pins the comparison as equality of the whole
// string.
//
// A prefix is the tempting mistake, and it is tempting in both directions: a
// target asking for "image/" reads as one that takes any image, and a source
// offering "image/png" reads as one that satisfies it. Neither is true here,
// and neither can become true quietly -- a source offering "text/plain" would
// start feeding a target that asked for "text/", which understands markup and
// would be handed prose.
func TestTypesMeetWholeOrNotAtAll(t *testing.T) {
	const offered = "image/png"

	// The control first. A loop of "nothing happened" passes just as well when
	// nothing is routed at all, so the same staging has to be shown producing a
	// meeting before its silence means anything.
	t.Run("the same string", func(t *testing.T) {
		s := stage(offered, offered)
		s.drag()
		assertEvents(t, s.targetEvents(1, 0, offered), transfer.InitiateEvent{})
		s.drop()
		assertEvents(t, s.sourceEvents(1, offered), transfer.RequestEvent{Type: offered})
	})

	for _, asked := range []string{
		"image/",     // the offered type begins with this
		"image",      // and with this
		"image/png2", // this begins with the offered type
		"image/svg",  // and this shares only the family
	} {
		t.Run(asked, func(t *testing.T) {
			s := stage(offered, asked)

			s.drag()
			assertEvents(t, s.targetEvents(-1, 0, asked))

			// Dropped on a target that cannot take it, the gesture ends: the
			// source is never asked for the data, and what it hears instead is
			// the cancel.
			s.drop()
			assertEvents(t, s.sourceEvents(-1, offered), transfer.CancelEvent{})
			assertEvents(t, s.targetEvents(-1, 0, asked))
		})
	}
}

// TestDataEventCarriesWhatWasOffered follows the bytes from the source's hand
// to the target's, and asks that they be the same bytes rather than an equal
// copy.
func TestDataEventCarriesWhatWasOffered(t *testing.T) {
	const mediaType = "text/plain"
	const content = "what the source had"

	s := stage(mediaType, mediaType)
	s.drag()
	s.targetEvents(1, 0, mediaType)
	s.drop()
	assertEvents(t, s.sourceEvents(1, mediaType), transfer.RequestEvent{Type: mediaType})

	offered := newOffer(content)
	s.router.Source().Execute(transfer.OfferCmd{Tag: s.source, Type: mediaType, Data: offered})

	data, ok := s.targetEvents(-1, 0, mediaType)[0].(transfer.DataEvent)
	if !ok {
		t.Fatalf("the target was not handed a %T", transfer.DataEvent{})
	}
	if data.Type != mediaType {
		t.Errorf("the event says the data is %q, want %q", data.Type, mediaType)
	}

	reader := data.Open()
	if reader != io.ReadCloser(offered) {
		t.Fatalf("the event opened %v, want the reader the source offered, %v", reader, offered)
	}
	read, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading the offered data: %v", err)
	}
	if string(read) != content {
		t.Errorf("the target read %q, want %q", read, content)
	}
}

// TestDataNobodyOpenedIsClosedForThem is one half of the ownership rule.
//
// A target may ignore a drop -- it has every right to, and the frame it would
// have opened the data in is over by the time anyone could notice. Nothing on
// the source's side is waiting to hear about it either, so if the system did
// not close what nobody took, every refused drop would leak a file handle.
func TestDataNobodyOpenedIsClosedForThem(t *testing.T) {
	const mediaType = "text/plain"

	s := stage(mediaType, mediaType)
	s.drag()
	s.targetEvents(1, 0, mediaType)
	s.drop()
	s.sourceEvents(1, mediaType)

	offered := newOffer("ignored")
	s.router.Source().Execute(transfer.OfferCmd{Tag: s.source, Type: mediaType, Data: offered})

	// The target is handed the event and does not open it.
	if got := len(s.targetEvents(-1, 0, mediaType)); got == 0 {
		t.Fatal("the target was handed nothing to ignore")
	}
	if offered.closed {
		t.Fatal("the data was closed inside the frame it was still openable in")
	}

	s.router.Frame(&s.ops)
	if !offered.closed {
		t.Error("the data nobody opened was left open")
	}
}

// TestDataThatWasOpenedIsTheCallersToClose is the other half.
//
// Opening it takes it over. The system stops counting the reader as its own the
// moment it is handed out, because it cannot know when the target is finished
// with it: the bytes may be read on a goroutine of the target's own, long after
// the frame that delivered them is drawn and gone.
func TestDataThatWasOpenedIsTheCallersToClose(t *testing.T) {
	const mediaType = "text/plain"

	s := stage(mediaType, mediaType)
	s.drag()
	s.targetEvents(1, 0, mediaType)
	s.drop()
	s.sourceEvents(1, mediaType)

	offered := newOffer("taken")
	s.router.Source().Execute(transfer.OfferCmd{Tag: s.source, Type: mediaType, Data: offered})

	data, ok := s.targetEvents(-1, 0, mediaType)[0].(transfer.DataEvent)
	if !ok {
		t.Fatalf("the target was not handed a %T", transfer.DataEvent{})
	}
	reader := data.Open()

	s.router.Frame(&s.ops)
	if offered.closed {
		t.Fatal("data that was opened was closed underneath the target that took it")
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("closing the taken data: %v", err)
	}
	if !offered.closed {
		t.Error("closing the opened reader did not close the offered data")
	}
}
