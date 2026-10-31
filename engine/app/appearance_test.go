package app

import (
	"testing"

	"github.com/arandu-io/ayra/engine/widget"
)

// TestTheNativeCodesMapToAnAppearance fixes the code Android, iOS and macOS
// report: 1 is light, 2 is dark, and anything else is a platform that did not
// say -- which must not be read as light.
func TestTheNativeCodesMapToAnAppearance(t *testing.T) {
	for code, want := range map[int]Appearance{
		0: AppearanceUnknown, 1: AppearanceLight, 2: AppearanceDark, 3: AppearanceUnknown, -1: AppearanceUnknown,
	} {
		if got := appearanceFromCode(code); got != want {
			t.Errorf("code %d is %v, want %v", code, got, want)
		}
	}
}

// TestTheWindowsSettingMapsToAnAppearance fixes the registry value: zero is the
// dark mode, and a value that could not be read is unknown rather than light.
func TestTheWindowsSettingMapsToAnAppearance(t *testing.T) {
	cases := []struct {
		value uint64
		read  bool
		want  Appearance
	}{
		{0, true, AppearanceDark},
		{1, true, AppearanceLight},
		{0, false, AppearanceUnknown},
		{1, false, AppearanceUnknown},
	}
	for _, c := range cases {
		if got := appearanceFromLightTheme(c.value, c.read); got != c.want {
			t.Errorf("value %d read=%v is %v, want %v", c.value, c.read, got, c.want)
		}
	}
}

// TestAChangeOfTheSystemSchemeIsDeliveredAtOnce keeps it from waiting for a
// frame. A configuration event that is not handled is held until something
// else flushes, and a window that went dark in the background has no frame to
// flush it with.
func TestAChangeOfTheSystemSchemeIsDeliveredAtOnce(t *testing.T) {
	w := new(Window)
	w.decorations.Decorations = new(widget.Decorations)
	if !w.processEvent(ConfigEvent{Config: Config{Appearance: AppearanceDark}}) {
		t.Error("a new appearance was held for the next frame")
	}
	if w.processEvent(ConfigEvent{Config: Config{Appearance: AppearanceDark}}) {
		t.Error("the same appearance asked for a delivery of its own")
	}
	if !w.processEvent(StageEvent{Stage: StageBackground}) {
		t.Error("the stage was held for the next frame")
	}
	e, ok := w.nextEvent()
	if !ok {
		t.Fatal("nothing was delivered")
	}
	if cfg, isCfg := e.(ConfigEvent); !isCfg || cfg.Config.Appearance != AppearanceDark {
		t.Errorf("the first delivery was %#v, want the dark configuration", e)
	}
	if e, _ := w.nextEvent(); e != (StageEvent{Stage: StageBackground}) {
		t.Errorf("the second delivery was %#v, want the background stage", e)
	}
}
