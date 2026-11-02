package frame_test

import (
	"flag"
	"image"
	gocolor "image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// update rewrites the approved pictures instead of comparing against them.
//
// Reviewing the change is the point of this whole gate, so it is a flag and
// never an environment variable: a flag appears in the command somebody typed
// and in the shell history afterwards, and cannot be left set in a profile
// where it silently approves everything for a year.
var update = flag.Bool("update", false, "rewrite the approved pictures")

// The pictures are drawn with the fonts this repository carries rather than
// with whatever the machine has installed. A golden file compared against a
// system font is a golden file that passes on the machine it was made on and
// fails everywhere else, which teaches everyone to run with -update.
func fonts() []text.FontFace { return gofont.Collection() }

// TestScreensLookLikeTheyDid draws each screen and compares it with the
// picture that was approved.
//
// This is the gate a build does not give. Everything below the layout call --
// the shaper finding a face, a control computing a size, the GPU accepting the
// operations, a colour reaching the pixels it was meant to reach -- compiles
// whether or not it works. Three real faults were found the first time these
// pictures were looked at: nothing painted the window's ground, so the dark
// scheme drew light text on white; a button's label sat against its left edge;
// and the text field drew its placeholder through the button's own function,
// so centring one centred the other.
func TestScreensLookLikeTheyDid(t *testing.T) {
	for _, screen := range screens() {
		for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
			for _, density := range []float32{1, 2} {
				name := pictureName(screen.name, scheme, density)

				t.Run(name, func(t *testing.T) {
					got, err := frame.Draw(frame.Options{
						Width:   screen.width,
						Height:  screen.height,
						Density: density,
						Scheme:  scheme,
						Fonts:   fonts(),
					}, screen.draw())
					if err != nil {
						if noSurface(err) {
							t.Skipf("no drawing surface on this machine: %v", err)
						}
						t.Fatalf("the screen did not draw: %v", err)
					}

					compare(t, name, got)
				})
			}
		}
	}
}

// screen is one picture this gate keeps.
//
// draw returns a fresh widget per picture rather than one shared across them,
// because a control holds its state: the same button drawn into four pictures
// is one button that was hovered in the first of them.
type screen struct {
	name          string
	width, height int
	draw          func() ayra.Widget
}

func screens() []screen {
	return []screen{
		{name: "sign-in", width: 360, height: 420, draw: signIn},
		{name: "controls", width: 360, height: 560, draw: controls},
		{name: "surfaces", width: 360, height: 620, draw: surfaces},
		{name: "choices", width: 360, height: 620, draw: choices},
		{name: "select-form", width: 360, height: 420, draw: selectForm},
		{name: "form-fields", width: 360, height: 420, draw: formFields},
		{name: "segmented", width: 360, height: 200, draw: segmentedScreen},
		{name: "breadcrumb-phone", width: 390, height: 360, draw: breadcrumbScreen},
		{name: "indicators", width: 360, height: 500, draw: indicators},
		{name: "lists", width: 360, height: 560, draw: lists},
		{name: "forms", width: 360, height: 620, draw: forms},
		{name: "data", width: 360, height: 480, draw: data},
		{name: "shell", width: 520, height: 420, draw: shellScreen},
		{name: "content", width: 360, height: 480, draw: content},
		{name: "month", width: 360, height: 470, draw: month},
		{name: "browsing", width: 360, height: 560, draw: browsing},
		{name: "media", width: 360, height: 620, draw: media},
		{name: "video", width: 360, height: 600, draw: liveVideo},
		{name: "open", width: 360, height: 420, draw: openPanels},
	}
}

// openPanels draws the panels that are normally shut.
//
// Every other picture here catches a control at rest, and a panel at rest is a
// panel nobody can see inside. That is how a row inside one came to paint the
// page's own colour against a panel that is not the page's colour -- a block of
// the wrong shade on every entry of every menu and every option of every list,
// in the dark scheme, for as long as the pictures only ever showed the control
// that opens them.
func openPanels() ayra.Widget {
	var actions widget.Menu
	var trigger widget.Button
	var chosen widget.Select

	actions.Open()
	chosen.Choose(1)

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Open", widget.Heading, true),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.MenuProps{Entries: []string{"Rename", "Duplicate", "", "Delete"}}.
					Layout(c, &actions, func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonProps{Label: "Actions", Variant: widget.Outline}.Layout(c, &trigger)
					})
			},
			spacer(150),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SelectProps{
					Options:     []string{"Free", "Team", "Enterprise"},
					Placeholder: "Choose one",
				}.Layout(c, &chosen)
			},
		)
	}
}

// month draws the calendar, and the two fields that open one.
//
// The day is written into the props rather than read from the clock, which is
// the whole reason these pictures can be kept at all: a control that asked the
// machine what day it was would draw a different picture tomorrow and a
// different one again in another timezone.
func month() ayra.Widget {
	var days widget.Calendar
	var picker widget.DatePicker
	var lookup widget.Autocomplete

	today := time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)
	days.Select(today.AddDate(0, 0, 4))
	picker.Select(today)

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("September", widget.Heading, true),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.CalendarProps{Today: today, Monday: true}.Layout(c, &days)
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.DatePickerProps{Today: today, Placeholder: "Pick a date"}.Layout(c, &picker)
			},
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.AutocompleteProps{
					Entries:     []string{"Arandu", "Ayra", "Joaju", "Kyse"},
					Placeholder: "Search a repository",
				}.Layout(c, &lookup)
			},
		)
	}
}

// browsing draws a hierarchy with two of its branches open, and a listing.
func browsing() ayra.Widget {
	var tree widget.Tree
	var files widget.FilePicker

	tree.Expand([]int{0})
	tree.Expand([]int{0, 0})
	tree.Select([]int{0, 0, 1})

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Project", widget.Heading, true),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.TreeProps{Roots: []widget.TreeNode{
					{Label: "app", Children: []widget.TreeNode{
						{Label: "Http", Children: []widget.TreeNode{
							{Label: "Controllers"},
							{Label: "Middleware"},
						}},
						{Label: "Models"},
					}},
					{Label: "go.mod", Detail: "1.2 kB"},
				}}.Layout(c, &tree)
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.FilePickerProps{
					Root: "Files",
					Path: []string{"resources"},
					Entries: []widget.FileEntry{
						{Name: "views", Dir: true},
						{Name: "app.css", Size: 5186},
						{Name: "logo.svg", Size: 2048},
					},
				}.Layout(c, &files)
			},
		)
	}
}

// media draws the transport, a row of slides and a colour being chosen.
func media() ayra.Widget {
	var transport widget.Player
	var slides widget.Carousel
	var colours widget.ColourPicker

	slides.Show(1)
	colours.SetColour(gocolor.NRGBA{R: 0xE8, G: 0x7A, B: 0x33, A: 0xFF})

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Media", widget.Heading, true),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.PlayerProps{
					Title:    "A recording",
					Position: 97 * time.Second,
					Duration: 254 * time.Second,
					Playing:  true,
					Volume:   0.7,
					Speed:    1,
				}.Layout(c, &transport)
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.CarouselProps{Count: 4, Dots: true, Arrows: true}.Layout(c, &slides,
					func(c ayra.Context, index int) ayra.Dimensions {
						return widget.CardProps{}.Layout(c, func(c ayra.Context) ayra.Dimensions {
							return line(slideBody(index), widget.Body, false)(c)
						})
					})
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.ColourPickerProps{}.Layout(c, &colours)
			},
		)
	}
}

// liveVideo draws a live view three ways: with a picture and a box over it, with
// a picture that has stopped and says so, and with no picture yet.
//
// The picture is computed here, pixel by pixel, rather than read from a file:
// there is no decoder in a headless run, and the point of the widget being given
// its picture in the props is that a test can hand it one.
func liveVideo() ayra.Widget {
	var running, stopped, waiting widget.Video
	scene := cameraScene(192, 108)

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Live", widget.Heading, true),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.VideoProps{
					Frame: scene, Seq: 1, Label: "Gate camera",
					Overlay: func(c ayra.Context, area widget.VideoArea) {
						box := area.Rect(image.Rect(100, 52, 156, 78))
						outline(c, box, gocolor.NRGBA{R: 0x22, G: 0xC5, B: 0x5E, A: 0xFF}, 2)
					},
				}.Layout(c, &running)
			},
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.VideoProps{Frame: scene, Seq: 1, Label: "Yard camera", Caption: "No signal from the camera"}.Layout(c, &stopped)
			},
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.VideoProps{Label: "Dock camera", Caption: "Preparing the video"}.Layout(c, &waiting)
			},
		)
	}
}

// cameraScene is a synthetic picture: a sky, a road and a vehicle, with
// gradients so that a scaling or a channel mix-up shows.
func cameraScene(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var px gocolor.NRGBA
			switch {
			case y < h/2:
				px = gocolor.NRGBA{R: uint8(60 + y), G: uint8(110 + y), B: uint8(190 + x/8), A: 255}
			default:
				v := uint8(70 + (y-h/2)/2)
				px = gocolor.NRGBA{R: v, G: v, B: v + 6, A: 255}
			}
			if x > 100 && x < 156 && y > 52 && y < 78 {
				px = gocolor.NRGBA{R: 200, G: 40, B: 40, A: 255}
			}
			img.Set(x, y, px)
		}
	}
	return img
}

// outline strokes a rectangle with four fills.
func outline(c ayra.Context, r image.Rectangle, colour gocolor.NRGBA, width int) {
	for _, side := range []image.Rectangle{
		{Min: r.Min, Max: image.Pt(r.Max.X, r.Min.Y+width)},
		{Min: image.Pt(r.Min.X, r.Max.Y-width), Max: r.Max},
		{Min: r.Min, Max: image.Pt(r.Min.X+width, r.Max.Y)},
		{Min: image.Pt(r.Max.X-width, r.Min.Y), Max: r.Max},
	} {
		paint.FillShape(c.Ops, colour, clip.Rect(side).Op())
	}
}

// slideBody is what each slide of the carousel says, so the pictures show which
// one the window is on.
func slideBody(index int) string {
	return []string{"The first", "The second", "The third", "The fourth"}[index]
}

// content draws a menu, a search result with its match marked, and a feed.
func content() ayra.Widget {
	var actions widget.Menu
	var trigger widget.Button

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Content", widget.Heading, true),
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return widget.HighlightProps{
					Text:  "Arandu draws its own controls",
					Match: "draws",
				}.Layout(c)
			},
			spacer(6),
			func(c ayra.Context) ayra.Dimensions {
				return widget.HighlightProps{
					Text:  "No match in this line",
					Match: "absent",
					Role:  widget.Caption,
				}.Layout(c)
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.FeedProps{Count: 3}.Layout(c, func(index int) ayra.Widget {
					entries := []struct{ title, body string }{
						{"Invited", "Three days ago"},
						{"Signed in", "Yesterday"},
						{"Changed the plan", "An hour ago"},
					}
					if index >= len(entries) {
						return nil
					}
					return func(c ayra.Context) ayra.Dimensions {
						return widget.ItemProps{Title: entries[index].title, Body: entries[index].body}.
							Layout(c, &widget.Button{}, nil)
					}
				})
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.MenuProps{Entries: []string{"Rename", "Duplicate", "", "Delete"}}.
					Layout(c, &actions, func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonProps{Label: "Actions", Variant: widget.Outline, Size: widget.Small}.Layout(c, &trigger)
					})
			},
		)
	}
}

// shellScreen draws the pieces an application is framed with: a sidebar, a
// segmented filter, a split action and a copy control.
func shellScreen() ayra.Widget {
	var nav widget.Sidebar
	nav.Show(1)
	var filter widget.Segments
	filter.Choose(0)
	var action widget.Split
	var copier widget.Copier

	return func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
				return widget.SidebarProps{
					Title:   "Workspace",
					Entries: []string{"Overview", "Members", "Billing", "Settings"},
					Width:   160,
				}.Layout(c.With(gtx), &nav)
			}),
			layout.Flexed(1, func(gtx layout.Context) ayra.Dimensions {
				inner := c.With(gtx)
				return layout.Inset{Left: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return column(inner.With(gtx), 320,
						line("Members", widget.Heading, true),
						spacer(10),
						func(c ayra.Context) ayra.Dimensions {
							return widget.SegmentedProps{Options: []string{"All", "Admins"}}.Layout(c, &filter)
						},
						spacer(12),
						row(
							func(c ayra.Context) ayra.Dimensions {
								return widget.SplitProps{Label: "Invite"}.Layout(c, &action)
							},
							func(c ayra.Context) ayra.Dimensions {
								return widget.CopyProps{Value: "https://example.com/join", Size: widget.Small}.Layout(c, &copier)
							},
						),
					)
				})
			}),
		)
	}
}

// data draws a toolbar over a table, a slider and a rating -- the shapes a
// screen of records is built from.
func data() ayra.Widget {
	var search widget.Input
	var add widget.Button
	var level widget.Slider
	level.SetValue(0.35)
	var stars widget.Rating
	stars.SetValue(4)

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 330,
			line("Data", widget.Heading, true),
			func(c ayra.Context) ayra.Dimensions {
				return widget.ToolbarProps{Divided: true}.Layout(c,
					func(c ayra.Context) ayra.Dimensions {
						return widget.InputProps{Placeholder: "Search"}.Layout(c, &search)
					},
					func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonProps{Label: "Add", Size: widget.Small}.Layout(c, &add)
					},
				)
			},
			func(c ayra.Context) ayra.Dimensions {
				return widget.TableProps{
					Columns: []widget.Column{
						{Title: "Name", Flex: 2},
						{Title: "Seats", Align: widget.End, Numeric: true},
						{Title: "Spend", Align: widget.End, Numeric: true},
					},
					Rows: [][]string{
						{"Arandu", "8", "1,240"},
						{"Peráta", "3", "890"},
						{"Joaju", "12", "12,004"},
					},
					Dense: true,
				}.Layout(c)
			},
			spacer(16),
			func(c ayra.Context) ayra.Dimensions {
				return widget.LabelProps{Text: "Threshold"}.Layout(c)
			},
			spacer(4),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SliderProps{}.Layout(c, &level)
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.LabelProps{Text: "Rating"}.Layout(c)
			},
			spacer(4),
			func(c ayra.Context) ayra.Dimensions {
				return widget.RatingProps{}.Layout(c, &stars)
			},
		)
	}
}

// forms draws a field with its error, a revealed password, a pager, a number
// stepper and a section that opens.
func forms() ayra.Widget {
	var email widget.Input
	var secret widget.Secret
	secret.SetText("correct horse")
	var pages widget.Pages
	pages.Show(4)
	var count widget.Stepper
	count.SetValue(3)
	var section widget.Disclosure
	section.Open()
	var picked widget.Select
	picked.Choose(1)

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Forms", widget.Heading, true),
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				props := widget.FieldProps{Label: "Email", Required: true, Error: "That address has no domain."}
				return props.Layout(c, func(c ayra.Context) ayra.Dimensions {
					return widget.InputProps{Placeholder: "you@example.com", Kind: widget.Email, Invalid: props.Invalid()}.Layout(c, &email)
				})
			},
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return widget.FieldProps{Label: "Password", Help: "At least twelve characters."}.Layout(c, func(c ayra.Context) ayra.Dimensions {
					return widget.PasswordProps{Placeholder: "Your password"}.Layout(c, &secret)
				})
			},
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return widget.FieldProps{Label: "Plan"}.Layout(c, func(c ayra.Context) ayra.Dimensions {
					return widget.SelectProps{Options: []string{"Free", "Team", "Enterprise"}, Placeholder: "Choose one"}.Layout(c, &picked)
				})
			},
			spacer(12),
			row(
				func(c ayra.Context) ayra.Dimensions {
					return widget.LabelProps{Text: "Seats"}.Layout(c)
				},
				func(c ayra.Context) ayra.Dimensions {
					return widget.NumberProps{Min: 1, Max: 10}.Layout(c, &count)
				},
			),
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.CollapsibleProps{Summary: "Advanced"}.Layout(c, &section, func(c ayra.Context) ayra.Dimensions {
					return line("Nothing here needs changing.", widget.Caption, false)(c)
				})
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.PaginationProps{Total: 12}.Layout(c, &pages)
			},
		)
	}
}

// lists draws the pieces a page of records is made of: a trail back, rows, a
// meter, figures, a joined set of buttons, and the message a page with nothing
// on it shows.
func lists() ayra.Widget {
	var crumbs widget.Crumbs
	var group widget.Group
	rows := make([]widget.Button, 3)
	var link widget.Button

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			func(c ayra.Context) ayra.Dimensions {
				return widget.BreadcrumbProps{Steps: []string{"Projects", "Arandu", "Members"}}.Layout(c, &crumbs)
			},
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return widget.ButtonGroupProps{Labels: []string{"All", "Admins", "Invited"}, Selected: 1}.Layout(c, &group)
			},
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return widget.ItemProps{Title: "Paulo Lima", Body: "Owner", Pressable: true, Selected: true}.Layout(c, &rows[0], func(c ayra.Context) ayra.Dimensions {
					return widget.StatusProps{Label: "Active", Tone: widget.Accent}.Layout(c)
				})
			},
			func(c ayra.Context) ayra.Dimensions {
				return widget.ItemProps{Title: "Everton Alves", Body: "Invited three days ago", Pressable: true}.Layout(c, &rows[1], func(c ayra.Context) ayra.Dimensions {
					return widget.BadgeProps{Label: "Pending", Variant: widget.Outline}.Layout(c)
				})
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.LabelProps{Text: "Seats used"}.Layout(c)
			},
			spacer(6),
			func(c ayra.Context) ayra.Dimensions {
				return widget.MeterProps{Value: 8, High: 10}.Layout(c)
			},
			spacer(14),
			row(
				func(c ayra.Context) ayra.Dimensions {
					return widget.StatProps{Value: "8", Label: "Members", Note: "of 10 seats"}.Layout(c)
				},
				func(c ayra.Context) ayra.Dimensions {
					return widget.StatProps{Value: "1.2k", Label: "Requests today"}.Layout(c)
				},
			),
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.LinkProps{Text: "Read the billing guide"}.Layout(c, &link)
			},
		)
	}
}

// indicators draws the pieces that report rather than accept: a label, a
// progress bar, an avatar, a placeholder, a key and a status.
func indicators() ayra.Widget {
	var tabs widget.Tabs
	tabs.Select(1)
	var note widget.Input

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Indicators", widget.Heading, true),
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return widget.TabsProps{Labels: []string{"Overview", "Members", "Billing"}}.Layout(c, &tabs)
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.LabelProps{Text: "Display name", Required: true}.Layout(c)
			},
			spacer(6),
			func(c ayra.Context) ayra.Dimensions {
				return widget.TextareaProps{Placeholder: "A sentence or two", Rows: 3}.Layout(c, &note)
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.ProgressProps{Fraction: 0.62}.Layout(c)
			},
			spacer(14),
			row(
				func(c ayra.Context) ayra.Dimensions {
					return widget.AvatarProps{Initials: "PL"}.Layout(c)
				},
				func(c ayra.Context) ayra.Dimensions {
					return widget.StatusProps{Label: "Active", Tone: widget.Accent}.Layout(c)
				},
				func(c ayra.Context) ayra.Dimensions {
					return widget.KbdProps{Keys: "Ctrl K"}.Layout(c)
				},
			),
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SkeletonProps{Width: 200}.Layout(c)
			},
			spacer(6),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SkeletonProps{Width: 140}.Layout(c)
			},
		)
	}
}

// row lays children out side by side with a gap between each.
func row(children ...ayra.Widget) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		items := make([]layout.FlexChild, 0, len(children)*2)
		for index, child := range children {
			child := child
			if index > 0 {
				items = append(items, layout.Rigid(layout.Spacer{Width: 12}.Layout))
			}
			items = append(items, layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
				return child(c.With(gtx))
			}))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, items...)
	}
}

// surfaces draws the pieces a screen is composed out of rather than the ones a
// person operates: a card, the four severities of an alert, the badges and a
// rule.
func surfaces() ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Surfaces", widget.Heading, true),
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return widget.CardProps{}.Layout(c, func(c ayra.Context) ayra.Dimensions {
					return column(c, 280,
						line("A card", widget.Body, true),
						spacer(6),
						line("It is a surface and a boundary, and carries whatever is written into it.", widget.Caption, false),
					)
				})
			},
			spacer(12),
			alert(widget.AlertProps{Title: "Two people are editing this", Severity: widget.Note}),
			spacer(8),
			alert(widget.AlertProps{Title: "Could not reach the server", Body: "It answered nothing for thirty seconds.", Severity: widget.Failure}),
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SeparatorProps{}.Layout(c)
			},
			spacer(12),
			badges(),
		)
	}
}

// choices draws the three controls that are either on or off, in both states
// and disabled.
func choices() ayra.Widget {
	states := make([]widget.Toggle, 10)
	states[1].Set(true)
	states[3].Set(true)
	states[5].Set(true)
	states[7].Set(true)

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Choices", widget.Heading, true),
			spacer(12),
			check(widget.CheckboxProps{Label: "Remember this device"}, &states[0]),
			spacer(8),
			check(widget.CheckboxProps{Label: "Send me the receipt"}, &states[1]),
			spacer(8),
			check(widget.CheckboxProps{Label: "Unavailable", Disabled: true}, &states[6]),
			spacer(8),
			check(widget.CheckboxProps{Label: "Some of the cameras", Mixed: true}, &states[8]),
			spacer(8),
			check(widget.CheckboxProps{Label: "I agree to the terms of use, the privacy policy and the sharing of the plate reads with the operator"}, &states[9]),
			spacer(14),
			radio(widget.RadioProps{Label: "Monthly"}, &states[2]),
			spacer(8),
			radio(widget.RadioProps{Label: "Yearly"}, &states[3]),
			spacer(14),
			toggle(widget.SwitchProps{Label: "Notifications"}, &states[4]),
			spacer(8),
			toggle(widget.SwitchProps{Label: "Two-step sign in"}, &states[5]),
			spacer(8),
			toggle(widget.SwitchProps{Label: "Unavailable", Disabled: true}, &states[7]),
		)
	}
}

// formFields draws a text field, a select, a day and a period one under the other,
// which is a filter: they take the same fill, the same border and the same height,
// so a row of them is one line.
func formFields() ayra.Widget {
	var name widget.Input
	var plan widget.Select
	plan.Choose(1)
	var day widget.DatePicker
	day.Select(reference)
	var period widget.DateRange
	var moment widget.DateTime

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Fields", widget.Heading, true),
			spacer(10),
			field(widget.InputProps{Placeholder: "Plate"}, &name),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SelectProps{Options: []string{"Free", "Team", "Enterprise"}, Placeholder: "Plan"}.Layout(c, &plan)
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.DatePickerProps{Today: reference, DateFormat: "02/01/2006"}.Layout(c, &day)
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.DateRangePickerProps{Today: reference}.Layout(c, &period)
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.DateTimePickerProps{Today: reference}.Layout(c, &moment)
			},
		)
	}
}

// breadcrumbScreen draws a trail across a phone and the same trail in a slice of
// 120 points, where a step has its least room and is cut with an ellipsis rather
// than broken a letter to a line.
func breadcrumbScreen() ayra.Widget {
	var wide, narrow widget.Crumbs
	steps := []string{"Workspace", "Projects", "Ayra", "Feedback", "Settings"}

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 358,
			line("Phone", widget.Heading, true),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.BreadcrumbProps{Steps: steps}.Layout(c, &wide)
			},
			spacer(18),
			line("120 points", widget.Heading, true),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return column(c, 120, func(c ayra.Context) ayra.Dimensions {
					return widget.BreadcrumbProps{Steps: []string{"Workspaces of the account", "Feedback", "Settings and billing"}}.Layout(c, &narrow)
				})
			},
		)
	}
}

// segmentedScreen draws the bar of choices with the first chosen and with the
// last, which is what tells whether the choice can be seen.
func segmentedScreen() ayra.Widget {
	var theme, days widget.Segments
	days.Choose(2)

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Segmented", widget.Heading, true),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SegmentedProps{Options: []string{"Automático", "Claro", "Escuro"}}.Layout(c, &theme)
			},
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SegmentedProps{Options: []string{"Dom", "Seg", "Ter", "Qua", "Qui"}}.Layout(c, &days)
			},
		)
	}
}

// selectForm draws a select that is open in the middle of a form, with the
// fields that come after it: the list covers them, and they do not cover it.
func selectForm() ayra.Widget {
	var plan widget.Select
	var after widget.Select
	var name widget.Input
	plan.Open()

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Form", widget.Heading, true),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SelectProps{Options: []string{"Free", "Team", "Enterprise"}, Placeholder: "Choose a plan"}.Layout(c, &plan)
			},
			spacer(6),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SelectProps{Options: []string{"Monthly", "Yearly"}, Placeholder: "Billing"}.Layout(c, &after)
			},
			spacer(6),
			field(widget.InputProps{Placeholder: "Name"}, &name),
		)
	}
}

func alert(props widget.AlertProps) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions { return props.Layout(c) }
}

func check(props widget.CheckboxProps, state *widget.Toggle) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions { return props.Layout(c, state) }
}

func radio(props widget.RadioProps, state *widget.Toggle) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions { return props.Layout(c, state) }
}

func toggle(props widget.SwitchProps, state *widget.Toggle) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions { return props.Layout(c, state) }
}

// badges draws one of each variant in a row, which is what catches a pair that
// look the same.
func badges() ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
				return widget.BadgeProps{Label: "Live"}.Layout(c.With(gtx))
			}),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
				return widget.BadgeProps{Label: "Draft", Variant: widget.Secondary}.Layout(c.With(gtx))
			}),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
				return widget.BadgeProps{Label: "Beta", Variant: widget.Outline}.Layout(c.With(gtx))
			}),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
				return widget.BadgeProps{Label: "Overdue", Variant: widget.Destructive}.Layout(c.With(gtx))
			}),
		)
	}
}

// signIn is the screen a person meets first.
func signIn() ayra.Widget {
	var email, password widget.Input
	var submit widget.Button

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 300,
			line("Sign in", widget.Heading, true),
			spacer(16),
			field(widget.InputProps{Placeholder: "you@example.com", Kind: widget.Email}, &email),
			spacer(12),
			field(widget.InputProps{Placeholder: "Your password", Kind: widget.Password}, &password),
			spacer(16),
			button(widget.ButtonProps{Label: "Sign in", Size: widget.Large}, &submit),
		)
	}
}

// controls draws one of every variant, which is what catches a colour that
// reads on one scheme and vanishes on the other.
func controls() ayra.Widget {
	states := make([]widget.Button, 7)
	var disabled, invalid widget.Input

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Controls", widget.Heading, true),
			spacer(12),
			button(widget.ButtonProps{Label: "Default"}, &states[0]),
			spacer(8),
			button(widget.ButtonProps{Label: "Secondary", Variant: widget.Secondary}, &states[1]),
			spacer(8),
			button(widget.ButtonProps{Label: "Outline", Variant: widget.Outline}, &states[2]),
			spacer(8),
			button(widget.ButtonProps{Label: "Ghost", Variant: widget.Ghost}, &states[3]),
			spacer(8),
			button(widget.ButtonProps{Label: "Destructive", Variant: widget.Destructive}, &states[4]),
			spacer(8),
			button(widget.ButtonProps{Label: "Link", Variant: widget.Link}, &states[5]),
			spacer(8),
			button(widget.ButtonProps{Label: "Unavailable", Disabled: true}, &states[6]),
			spacer(16),
			field(widget.InputProps{Placeholder: "Unavailable", Disabled: true}, &disabled),
			spacer(8),
			field(widget.InputProps{Placeholder: "Rejected", Invalid: true}, &invalid),
		)
	}
}

// The pieces the screens above are built from. They are the test's own, and
// not the library's: a helper shared with the code under test would draw the
// same mistake on both sides of the comparison.

func line(content string, role widget.Role, bold bool) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		return widget.TextProps{Content: content, Role: role, Bold: bold}.Layout(c)
	}
}

func field(props widget.InputProps, state *widget.Input) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions { return props.Layout(c, state) }
}

func button(props widget.ButtonProps, state *widget.Button) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions { return props.Layout(c, state) }
}

func spacer(height unit.Dp) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Spacer{Height: height}.Layout(c.Context)
	}
}

func column(c ayra.Context, width unit.Dp, children ...ayra.Widget) ayra.Dimensions {
	return layout.Center.Layout(c.Context, func(gtx layout.Context) ayra.Dimensions {
		if max := c.Dp(width); gtx.Constraints.Max.X > max {
			gtx.Constraints.Max.X = max
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		inner := c.With(gtx)

		items := make([]layout.FlexChild, 0, len(children))
		for _, child := range children {
			child := child
			items = append(items, layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
				return child(inner.With(gtx))
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(inner.Context, items...)
	})
}

// pictureName is what a picture is filed under: the screen, the scheme and the
// density, in that order, so the directory sorts by screen.
func pictureName(screen string, scheme theme.Scheme, density float32) string {
	suffix := "1x"
	if density == 2 {
		suffix = "2x"
	}
	return screen + "-" + scheme.String() + "-" + suffix
}

// compare checks a drawing against the approved picture, or writes it when
// -update was passed.
func compare(t *testing.T, name string, got *image.RGBA) {
	t.Helper()

	path := filepath.Join("testdata", name+".png")

	if *update {
		write(t, path, got)
		return
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("%s has no approved picture: draw it with `go test ./frame -update` and look at it before committing", name)
	}
	defer file.Close()

	decoded, err := png.Decode(file)
	if err != nil {
		t.Fatalf("%s could not be read: %v", path, err)
	}

	if diff := difference(got, decoded); diff > 0 {
		failed := filepath.Join("testdata", name+".failed.png")
		write(t, failed, got)
		t.Errorf("%s changed in %d pixels.\n  approved: %s\n  drawn:    %s\nLook at both. If the change is the one you meant, `go test ./frame -update`.",
			name, diff, path, failed)
	}
}

// tolerance is how far one channel may differ before a pixel counts as
// changed, out of 255.
//
// It is not a comfort margin, it is the width of a measured gap. The same
// screens drawn by this machine's GPU and by the software renderer a build
// server uses differ in five percent of their pixels, and the measurement of
// that difference is decisive: ninety-nine percent of it is three or less, the
// largest is nine, and across all eight pictures not one pixel differs by more
// than sixteen. A real change is not in that range at all -- text that moved,
// a ground that was not painted and a rule under a word are all whole colours
// replacing whole colours.
//
// Comparing exactly would mean approved pictures that pass only on the machine
// that drew them, which teaches everyone to run with -update, which is the same
// as having no gate. Comparing loosely by a percentage of the picture would
// hide the small true changes: the rule under a link is twenty-seven pixels.
const tolerance = 16

// difference counts the pixels that do not match.
//
// A count rather than a boolean, because the number is the first thing worth
// knowing: four pixels is an anti-aliased edge that moved, and forty thousand
// is a screen that went blank.
func difference(got *image.RGBA, want image.Image) int {
	if got.Bounds() != want.Bounds() {
		return got.Bounds().Dx() * got.Bounds().Dy()
	}

	var count int
	bounds := got.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if changed(got.At(x, y), want.At(x, y)) {
				count++
			}
		}
	}
	return count
}

// changed reports whether two pixels differ by more than the tolerance in any
// channel.
func changed(got, want gocolor.Color) bool {
	gr, gg, gb, ga := got.RGBA()
	wr, wg, wb, wa := want.RGBA()

	for _, pair := range [4][2]uint32{{gr, wr}, {gg, wg}, {gb, wb}, {ga, wa}} {
		// RGBA answers sixteen bits per channel; the tolerance is in eight, so
		// both sides come down before the subtraction rather than the constant
		// going up, which would compare a rounded number against an exact one.
		first, second := int(pair[0]>>8), int(pair[1]>>8)
		if first-second > tolerance || second-first > tolerance {
			return true
		}
	}
	return false
}

// write saves a picture, creating the directory if this is the first one.
func write(t *testing.T, path string, picture *image.RGBA) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if err := png.Encode(file, picture); err != nil {
		t.Fatal(err)
	}
}

// noSurface reports whether the failure is a machine with nowhere to draw
// rather than a fault in the drawing.
//
// A machine without a GPU cannot run this gate, and there is no software
// rasteriser to fall back to. Skipping says so; failing would teach whoever
// hits it that the gate is unreliable, which is how a red suite gets ignored.
func noSurface(err error) bool {
	return strings.Contains(err.Error(), "no drawing surface")
}
