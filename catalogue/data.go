package main

import (
	"strconv"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// data are the controls that show a set of something: rows, days, files,
// a recording.
func data() []Entry {
	return []Entry{
		tableEntry(),
		feedEntry(),
		calendarEntry(),
		datePickerEntry(),
		filePickerEntry(),
		playerEntry(),
	}
}

// today is the day the catalogue was opened, taken down to its own midnight.
//
// It is read here, once, and handed to every control that needs a day. The
// controls take it as a value rather than asking the machine, so that what they
// draw can be pinned by a test; a program is exactly the place the question is
// allowed to be asked, and asking it once means two calendars on one screen
// agree about what day it is.
func today() time.Time {
	now := time.Now()
	year, month, day := now.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, now.Location())
}

// tableEntry draws the grid, at every alignment and in both densities.
func tableEntry() Entry {
	return Entry{
		Control: "TableProps",
		Summary: "A grid of values with a heading, whose shape it owns and whose rows it does not.",
		Build: func() ayra.Widget {
			columns := []widget.Column{
				{Title: "Workspace", Flex: 2},
				{Title: "State", Align: widget.Middle},
				{Title: "Seats", Align: widget.End, Numeric: true},
				{Title: "Spend", Align: widget.End, Numeric: true},
			}
			rows := [][]string{
				{"Arandu", "Running", "8", "1,240"},
				{"Perátā", "Stopped", "3", "890"},
				{"Kyse", "Running", "27", "12,405"},
				{"Joaju", "Failed", "1"},
			}

			return func(c ayra.Context) ayra.Dimensions {
				return stack(20,
					labelled("the room a table is operated at", sized(560, func(c ayra.Context) ayra.Dimensions {
						return widget.TableProps{Columns: columns, Rows: rows}.Layout(c)
					})),
					labelled("dense, for one that is read", sized(560, func(c ayra.Context) ayra.Dimensions {
						return widget.TableProps{Columns: columns, Rows: rows, Dense: true}.Layout(c)
					})),
					note("the last row is a field short, and draws a blank rather than failing"),
				)(c)
			}
		},
	}
}

// feedEntry draws the column of entries against its rail.
func feedEntry() Entry {
	return Entry{
		Control: "FeedProps",
		Summary: "Entries in time order against a rail; what an entry is belongs to the caller.",
		Build: func() ayra.Widget {
			entries := []struct{ title, body string }{
				{"Workspace opened", "Three weeks ago"},
				{"Two members invited", "Nine days ago"},
				{"Plan changed to yearly", "Yesterday"},
				{"Invoice paid", "An hour ago"},
			}
			rows := make([]widget.Button, len(entries))

			return func(c ayra.Context) ayra.Dimensions {
				return sized(420, func(c ayra.Context) ayra.Dimensions {
					return widget.FeedProps{Count: len(entries)}.Layout(c, func(index int) ayra.Widget {
						if index >= len(entries) {
							return nil
						}
						return func(c ayra.Context) ayra.Dimensions {
							return widget.ItemProps{
								Title: entries[index].title,
								Body:  entries[index].body,
							}.Layout(c, &rows[index], nil)
						}
					})
				})(c)
			}
		},
	}
}

// calendarEntry draws the month, on both first days of the week and with the
// choosable range bounded.
func calendarEntry() Entry {
	return Entry{
		Control: "CalendarProps",
		Summary: "A month of days, drawn from the day it was handed rather than from a clock.",
		Build: func() ayra.Widget {
			now := today()

			var sunday, monday, bounded widget.Calendar
			sunday.Select(now)
			monday.Select(now.AddDate(0, 0, 3))
			bounded.Show(now)

			return func(c ayra.Context) ayra.Dimensions {
				chosen := "nothing"
				if day := sunday.Selected(); !day.IsZero() {
					chosen = day.Format(widget.DateFormat)
				}

				return stack(16,
					flow(24,
						labelled("weeks beginning on Sunday", sized(280, func(c ayra.Context) ayra.Dimensions {
							return widget.CalendarProps{Today: now}.Layout(c, &sunday)
						})),
						labelled("weeks beginning on Monday", sized(280, func(c ayra.Context) ayra.Dimensions {
							return widget.CalendarProps{Today: now, Monday: true}.Layout(c, &monday)
						})),
						labelled("bounded to the week around today", sized(280, func(c ayra.Context) ayra.Dimensions {
							return widget.CalendarProps{
								Today: now,
								Min:   now.AddDate(0, 0, -3),
								Max:   now.AddDate(0, 0, 3),
							}.Layout(c, &bounded)
						})),
					),
					note("the first one holds: "+chosen),
				)(c)
			}
		},
	}
}

// datePickerEntry draws the field that opens a month, in both directions and
// unavailable.
func datePickerEntry() Entry {
	return Entry{
		Control: "DatePickerProps",
		Summary: "A field that opens a month, writing the day in an order nobody misreads.",
		Build: func() ayra.Widget {
			now := today()

			var below, above, off widget.DatePicker
			off.Select(now.AddDate(0, 0, -10))
			picked := "nothing yet"

			return func(c ayra.Context) ayra.Dimensions {
				if below.Changed() {
					picked = below.Selected().Format(widget.DateFormat)
				}
				if above.Changed() {
					picked = above.Selected().Format(widget.DateFormat)
				}

				return stack(16,
					labelled("opens downwards", sized(300, func(c ayra.Context) ayra.Dimensions {
						return widget.DatePickerProps{Today: now, Monday: true}.Layout(c, &below)
					})),
					labelled("opens upwards, and bounded to this month", sized(300, func(c ayra.Context) ayra.Dimensions {
						return widget.DatePickerProps{
							Placeholder: "When it is due",
							Today:       now,
							Min:         now.AddDate(0, 0, -now.Day()+1),
							Max:         now.AddDate(0, 1, -now.Day()),
							Above:       true,
						}.Layout(c, &above)
					})),
					labelled("disabled, and the month cannot be reached at all", sized(300, func(c ayra.Context) ayra.Dimensions {
						return widget.DatePickerProps{Today: now, Disabled: true}.Layout(c, &off)
					})),
					note("last picked: "+picked),
				)(c)
			}
		},
	}
}

// filePickerEntry draws the listing somebody browses, with the trail moving as
// they do.
//
// The move is made here, because the control reports where somebody asked to go
// and never moves itself: a picker that walked into a directory it could not
// read would be showing a place it is not.
func filePickerEntry() Entry {
	return Entry{
		Control: "FilePickerProps",
		Summary: "A listing somebody browses, which reads nothing and is handed everything.",
		Build: func() ayra.Widget {
			listings := map[string][]widget.FileEntry{
				"": {
					{Name: "app", Dir: true},
					{Name: "resources", Dir: true},
					{Name: "go.mod", Size: 1203},
					{Name: "go.sum", Size: 23144},
					{Name: "README.md", Size: -1},
				},
				"app": {
					{Name: "Http", Dir: true},
					{Name: "Models", Dir: true},
					{Name: "kernel.go", Size: 4096},
				},
				"resources": {
					{Name: "views", Dir: true},
					{Name: "app.css", Size: 5186},
					{Name: "logo.svg", Size: 2048},
				},
			}

			var browser, off widget.FilePicker
			path := []string{}
			chosen := "nothing yet"

			at := func() string {
				if len(path) == 0 {
					return ""
				}
				return path[len(path)-1]
			}

			return func(c ayra.Context) ayra.Dimensions {
				if name, ok := browser.Opened(); ok {
					if _, known := listings[name]; known {
						path = append(path, name)
					}
				}
				if depth, ok := browser.Up(); ok {
					path = path[:min(depth, len(path))]
				}
				if name, ok := browser.Chosen(); ok {
					chosen = name
				}

				return stack(16,
					flow(24,
						labelled("browsing", sized(320, func(c ayra.Context) ayra.Dimensions {
							return widget.FilePickerProps{
								Root:    "Files",
								Path:    path,
								Entries: listings[at()],
							}.Layout(c, &browser)
						})),
						labelled("disabled, and nothing is reported", sized(320, func(c ayra.Context) ayra.Dimensions {
							return widget.FilePickerProps{
								Root:     "Files",
								Entries:  listings[""],
								Disabled: true,
							}.Layout(c, &off)
						})),
					),
					note("last chosen: "+chosen+"; the README has no size and draws none"),
				)(c)
			}
		},
	}
}

// playerEntry draws the transport, and carries out what it asks for.
//
// The control decodes nothing and owns no clock: everything it draws is handed
// to it each frame, and everything a person asks of it comes back as an intent.
// Carrying those out is what this demonstration is -- without it the play
// button is a shape that lights up.
func playerEntry() Entry {
	return Entry{
		Control: "PlayerProps",
		Summary: "A transport that draws what it is told and reports what was asked of it.",
		Build: func() ayra.Widget {
			var full, compact, unknown, off widget.Player

			duration := 254 * time.Second
			position := 97 * time.Second
			playing := false
			muted := false
			volume := float32(0.7)
			speed := float32(1)

			return func(c ayra.Context) ayra.Dimensions {
				if full.Toggled() {
					playing = !playing
				}
				if full.MuteToggled() {
					muted = !muted
				}
				if at, ok := full.Sought(); ok {
					position = at
				}
				if level, ok := full.VolumeSet(); ok {
					volume = level
				}
				if rate, ok := full.SpeedSet(); ok {
					speed = rate
				}
				if at, scrubbing := full.Scrubbing(); scrubbing {
					position = at
				}

				return stack(18,
					labelled("with a title, and the rate it plays at", sized(480, func(c ayra.Context) ayra.Dimensions {
						return widget.PlayerProps{
							Title:     "A recording",
							Position:  position,
							Duration:  duration,
							Playing:   playing,
							Volume:    volume,
							Muted:     muted,
							Speed:     speed,
							ShowSpeed: true,
						}.Layout(c, &full)
					})),
					note("playing: "+strconv.FormatBool(playing)+
						", muted: "+strconv.FormatBool(muted)+
						", at "+position.Truncate(time.Second).String()+
						", speed "+strconv.FormatFloat(float64(speed), 'f', 2, 32)),
					labelled("compact, for a row of a list", sized(360, func(c ayra.Context) ayra.Dimensions {
						return widget.PlayerProps{
							Position: 31 * time.Second,
							Duration: 96 * time.Second,
							Volume:   0.5,
							Compact:  true,
						}.Layout(c, &compact)
					})),
					labelled("length not known yet, so every ratio is guarded", sized(480, func(c ayra.Context) ayra.Dimensions {
						return widget.PlayerProps{
							Title:    "A stream",
							Position: 12 * time.Second,
							Playing:  true,
							Volume:   1,
						}.Layout(c, &unknown)
					})),
					labelled("disabled", sized(480, func(c ayra.Context) ayra.Dimensions {
						return widget.PlayerProps{
							Title:    "A recording",
							Position: 40 * time.Second,
							Duration: duration,
							Volume:   0.3,
							Disabled: true,
						}.Layout(c, &off)
					})),
				)(c)
			}
		},
	}
}
