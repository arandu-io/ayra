package push

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fake is a platform: it records what it was asked and answers what it was told
// to, and the test plays the system by calling the hub as the bridge would.
type fake struct {
	kindName string
	answer   Permission
	err      error
	asked    int
	ensured  int
	release  chan struct{}
}

func (f *fake) kind() string { return f.kindName }

func (f *fake) ensure() { f.ensured++ }

func (f *fake) request(ctx context.Context) (Permission, error) {
	f.asked++
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return Undetermined, ctx.Err()
		}
	}
	return f.answer, f.err
}

// platform installs a backend and an empty hub for one test.
func platform(t *testing.T, f *fake) {
	t.Helper()
	oldBackend, oldHub := current, hub
	current, hub = f, newHub()
	t.Cleanup(func() { current, hub = oldBackend, oldHub })
}

func TestAPlatformWithNoPushSaysSo(t *testing.T) {
	platform(t, nil) // replaced below
	current = unsupported{}

	if Supported() || Kind() != "" {
		t.Errorf("an unsupported platform reports kind %q", Kind())
	}
	if _, err := Request(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Request answered %v, want ErrUnsupported", err)
	}
	if Token() != "" {
		t.Error("a token appeared from nowhere")
	}
}

func TestRequestAnswersWhatThePersonChose(t *testing.T) {
	for _, want := range []Permission{Granted, Denied} {
		f := &fake{kindName: "fcm", answer: want}
		platform(t, f)

		got, err := Request(context.Background())
		if err != nil || got != want {
			t.Errorf("Request answered %v, %v; want %v", got, err, want)
		}
		if !Supported() || Kind() != "fcm" {
			t.Errorf("kind %q", Kind())
		}
	}
}

func TestRequestPassesTheBackendsErrorsThrough(t *testing.T) {
	f := &fake{kindName: "fcm", err: ErrNotConfigured}
	platform(t, f)
	if _, err := Request(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Request answered %v", err)
	}
}

func TestRequestEndsWithItsContext(t *testing.T) {
	f := &fake{kindName: "apns", release: make(chan struct{})}
	platform(t, f)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	if _, err := Request(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Request answered %v, want the context's error", err)
	}
}

func TestATokenIsDeliveredOnceAndAgainWhenItChanges(t *testing.T) {
	platform(t, &fake{kindName: "fcm"})

	hub.deliverToken("t1")
	hub.deliverToken("t1") // the service repeats itself
	hub.deliverToken("")   // and sometimes says nothing
	hub.deliverToken("t2")

	if got := <-Tokens(); got != "t1" {
		t.Errorf("first token %q", got)
	}
	if got := <-Tokens(); got != "t2" {
		t.Errorf("second token %q", got)
	}
	select {
	case extra := <-Tokens():
		t.Errorf("an unchanged token was delivered again: %q", extra)
	default:
	}
	if Token() != "t2" {
		t.Errorf("Token is %q, want the latest", Token())
	}
}

// TestWhatArrivedBeforeAnyoneReadWaits is the notification that started the
// application: it lands before a window exists to read it.
func TestWhatArrivedBeforeAnyoneReadWaits(t *testing.T) {
	platform(t, &fake{kindName: "fcm"})

	hub.deliverOpened(Message{ID: "m1", Data: map[string]string{"link": "/painel/cameras/9"}})

	select {
	case m := <-Opened():
		if m.ID != "m1" || m.Link != "/painel/cameras/9" {
			t.Errorf("the opened message is %+v", m)
		}
	default:
		t.Fatal("the notification that opened the application was lost")
	}
}

func TestTheLinkIsTheFieldOrTheDataKey(t *testing.T) {
	platform(t, &fake{kindName: "fcm"})

	hub.deliverMessage(Message{Link: "arandu://a", Data: map[string]string{"link": "arandu://b"}})
	hub.deliverMessage(Message{Data: map[string]string{"link": "arandu://c"}})
	hub.deliverMessage(Message{})

	if got := (<-Messages()).Link; got != "arandu://a" {
		t.Errorf("an explicit link was replaced by %q", got)
	}
	if got := (<-Messages()).Link; got != "arandu://c" {
		t.Errorf("the link from the data was %q", got)
	}
	if got := (<-Messages()).Link; got != "" {
		t.Errorf("a message with no link got %q", got)
	}
}

// TestAFullQueueDropsTheOldestAndNeverBlocks is the rule that keeps a stuck
// screen from stopping the bridge: the delivery runs on a system thread.
func TestAFullQueueDropsTheOldestAndNeverBlocks(t *testing.T) {
	platform(t, &fake{kindName: "fcm"})

	done := make(chan struct{})
	go func() {
		for i := 0; i < queueSize+10; i++ {
			hub.deliverMessage(Message{ID: string(rune('A' + i%26)), Title: string(rune('0' + i%10))})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("delivering to a full queue blocked")
	}

	if got := len(Messages()); got != queueSize {
		t.Errorf("the queue holds %d, want %d", got, queueSize)
	}
	// The first ten were dropped: what is left starts at the eleventh.
	first := <-Messages()
	if first.ID != string(rune('A'+10%26)) {
		t.Errorf("the oldest kept is %q, want the eleventh sent", first.ID)
	}
}

func TestDeliveriesFromManyThreadsAreSafe(t *testing.T) {
	platform(t, &fake{kindName: "fcm"})
	done := make(chan struct{}, 8)
	for g := 0; g < 8; g++ {
		go func() {
			for i := 0; i < 200; i++ {
				hub.deliverMessage(Message{ID: "x"})
				hub.deliverOpened(Message{ID: "y"})
				hub.deliverToken("t")
			}
			done <- struct{}{}
		}()
	}
	for g := 0; g < 8; g++ {
		<-done
	}
}

func TestPermissionNamesItself(t *testing.T) {
	for p, want := range map[Permission]string{Undetermined: "undetermined", Granted: "granted", Denied: "denied"} {
		if p.String() != want {
			t.Errorf("%d is %q, want %q", p, p.String(), want)
		}
	}
}
