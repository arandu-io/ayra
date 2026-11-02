package widget_test

import (
	"image"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/widget"
)

// reachCase is a control and what a press on it changes.
type reachCase struct {
	name string
	// draw lays the control out and answers its room.
	draw func(c ayra.Context) ayra.Dimensions
	// centre is where the control is, given the room it reported, relative to
	// the control's own corner.
	centre func(room image.Point) image.Point
	// changed reports whether a press landed.
	changed func() bool
	// reset puts the state back for the next press.
	reset func()
	// short says the control is under 44 points tall, so that a press 20 points
	// off its middle is a finger and one 30 off is not.
	short bool
}

func middle(room image.Point) image.Point { return image.Pt(room.X/2, room.Y/2) }

// reachCases are the controls drawn smaller than a finger.
func reachCases() []reachCase {
	var slider widget.Slider
	var rating widget.Rating
	var box, flip widget.Toggle
	var pick widget.Select
	var date widget.DatePicker
	var period widget.DateRange
	var moment widget.DateTime
	var step widget.Stepper
	var month widget.Calendar
	today := time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)
	month.Show(today)

	return []reachCase{
		{
			name: "slider",
			draw: func(c ayra.Context) ayra.Dimensions {
				c.Constraints.Max.X = 200
				return widget.SliderProps{}.Layout(c, &slider)
			},
			centre:  middle,
			changed: func() bool { return slider.Value() != 0 },
			reset:   func() { slider.SetValue(0) },
		},
		{
			name:    "rating",
			draw:    func(c ayra.Context) ayra.Dimensions { return widget.RatingProps{}.Layout(c, &rating) },
			centre:  func(room image.Point) image.Point { return image.Pt(12, room.Y/2) },
			changed: func() bool { return rating.Value() != 0 },
			reset:   func() { rating.SetValue(0) },
		},
		{
			name:    "checkbox with no label",
			draw:    func(c ayra.Context) ayra.Dimensions { box.Changed(c); return widget.CheckboxProps{}.Layout(c, &box) },
			centre:  middle,
			changed: box.On,
			reset:   func() { box.Set(false) },
		},
		{
			name:    "switch with no label",
			draw:    func(c ayra.Context) ayra.Dimensions { flip.Changed(c); return widget.SwitchProps{}.Layout(c, &flip) },
			centre:  middle,
			changed: flip.On,
			reset:   func() { flip.Set(false) },
		},
		{
			name: "select",
			draw: func(c ayra.Context) ayra.Dimensions {
				c.Constraints.Max.X = 300
				return widget.SelectProps{Options: []string{"a"}}.Layout(c, &pick)
			},
			centre:  middle,
			changed: pick.Showing,
			reset:   func() { pick = widget.Select{} },
			short:   true,
		},
		{
			name: "date",
			draw: func(c ayra.Context) ayra.Dimensions {
				c.Constraints.Max.X = 300
				return widget.DatePickerProps{Today: today}.Layout(c, &date)
			},
			centre:  middle,
			changed: date.Showing,
			reset:   func() { date = widget.DatePicker{} },
			short:   true,
		},
		{
			name: "period",
			draw: func(c ayra.Context) ayra.Dimensions {
				c.Constraints.Max.X = 300
				return widget.DateRangePickerProps{Today: today}.Layout(c, &period)
			},
			centre:  middle,
			changed: period.Showing,
			reset:   func() { period = widget.DateRange{} },
			short:   true,
		},
		{
			name: "date and time",
			draw: func(c ayra.Context) ayra.Dimensions {
				c.Constraints.Max.X = 300
				return widget.DateTimePickerProps{Today: today}.Layout(c, &moment)
			},
			centre:  middle,
			changed: moment.Showing,
			reset:   func() { moment = widget.DateTime{} },
			short:   true,
		},
		{
			name:    "stepper's plus",
			draw:    func(c ayra.Context) ayra.Dimensions { return widget.NumberProps{}.Layout(c, &step) },
			centre:  func(room image.Point) image.Point { return image.Pt(room.X-14, room.Y/2) },
			changed: func() bool { return step.Value() != 0 },
			reset:   func() { step.SetValue(0) },
		},
		{
			name: "month's previous arrow",
			draw: func(c ayra.Context) ayra.Dimensions {
				c.Constraints.Max.X = 300
				return widget.CalendarProps{Today: today}.Layout(c, &month)
			},
			centre:  func(room image.Point) image.Point { return image.Pt(14, 14) },
			changed: func() bool { return month.Showing(today).Month() != time.September },
			reset:   func() { month.Show(today) },
		},
	}
}

// TestEveryControlSmallerThanAFingerIsPressedByOne is the touch area of the
// controls this group draws at the size Kyse draws them: a slider 20 points
// tall, a box 18, a stepper 28, a field 33. A finger lands within 22 points of the
// middle of each and the control has to answer; the room it reports, which is
// what a screen is laid out around, stays what it drew.
func TestEveryControlSmallerThanAFingerIsPressedByOne(t *testing.T) {
	for _, tc := range reachCases() {
		t.Run(tc.name, func(t *testing.T) {
			p := newPanel()
			screen := func(c ayra.Context) ayra.Dimensions {
				return layout.Inset{Top: 100, Left: 50}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
					return tc.draw(c.With(gtx))
				})
			}

			origin := image.Pt(50, 100)
			room := p.draw(screen).Size.Sub(origin)
			p.draw(screen)
			centre := origin.Add(tc.centre(room))

			// 20 points off the middle, above and below, is inside 44.
			for name, at := range map[string]image.Point{
				"above": centre.Sub(image.Pt(0, 20)),
				"below": centre.Add(image.Pt(0, 20)),
			} {
				tc.reset()
				p.tap(at, screen)
				if !tc.changed() {
					t.Errorf("a tap %s the control (room %v), 20 points from its middle, did not press it", name, room)
				}
			}

			tc.reset()
			p.tap(centre.Add(image.Pt(0, 30)), screen)
			if tc.changed() {
				t.Errorf("a tap 30 points from the middle (room %v) pressed it: the area is more than 44 tall", room)
			}
		})
	}
}
