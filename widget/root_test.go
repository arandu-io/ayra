package widget_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/icons"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// ADR-0131 gives the web components three tests, and this is the first of them
// for the native half: every component takes an override, and the override shows.
//
// A widget migrated by embedding ComponentProps and left drawing as it always did
// compiles, draws and ignores the caller in silence. The check is the same one:
// write the style, and require that it appears -- here as the room the widget
// takes, because a padding on the root is the one change every shape answers.

var restyle = widget.ComponentProps{Parts: theme.Parts{"root": {Padding: theme.Uniform(30)}}}

// each is one widget, drawn with the instance's ComponentProps handed to it.
type each struct {
	name string
	draw func(c ayra.Context, inst widget.ComponentProps) ayra.Dimensions
}

func noop(c ayra.Context) ayra.Dimensions { return ayra.Dimensions{} }

var day = time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)

func everyWidget() []each {
	return []each{
		{"badge", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.BadgeProps{Label: "x", ComponentProps: i}.Layout(c)
		}},
		{"alert", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.AlertProps{Title: "x", ComponentProps: i}.Layout(c)
		}},
		{"button", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.ButtonProps{Label: "x", ComponentProps: i}.Layout(c, new(widget.Button))
		}},
		{"card", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.CardProps{ComponentProps: i}.Layout(c, noop)
		}},
		{"input", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.InputProps{ComponentProps: i}.Layout(c, new(widget.Input))
		}},
		{"textarea", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.TextareaProps{ComponentProps: i}.Layout(c, new(widget.Input))
		}},
		{"masked", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.MaskedProps{ComponentProps: i}.Layout(c, new(widget.Masked))
		}},
		{"label", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.LabelProps{Text: "x", ComponentProps: i}.Layout(c)
		}},
		{"progress", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.ProgressProps{Fraction: .5, ComponentProps: i}.Layout(c)
		}},
		{"avatar", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.AvatarProps{Initials: "PL", ComponentProps: i}.Layout(c)
		}},
		{"video", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.VideoProps{Caption: "x", ComponentProps: i}.Layout(c, new(widget.Video))
		}},
		{"skeleton", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.SkeletonProps{ComponentProps: i}.Layout(c)
		}},
		{"kbd", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.KbdProps{Keys: "K", ComponentProps: i}.Layout(c)
		}},
		{"status", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.StatusProps{Label: "x", ComponentProps: i}.Layout(c)
		}},
		{"separator", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.SeparatorProps{ComponentProps: i}.Layout(c)
		}},
		{"onetimecode", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.OneTimeCodeProps{ComponentProps: i}.Layout(c, new(widget.OneTimeCode))
		}},
		{"daterangepicker", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.DateRangePickerProps{Today: day, ComponentProps: i}.Layout(c, new(widget.DateRange))
		}},
		{"datetimepicker", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.DateTimePickerProps{Today: day, ComponentProps: i}.Layout(c, new(widget.DateTime))
		}},
		{"datatable", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.DataTableProps{
				ComponentProps: i, Searchable: true, Mode: widget.ModeTable, // the cards fill the room they are given, which hides a padding
				Columns: []widget.DataColumn{{Key: "plate", Title: "Plate"}},
			}.Layout(c, new(widget.DataTable))
		}},
		{"combobox", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.ComboboxProps{ComponentProps: i}.Layout(c, new(widget.Combobox))
		}},
		{"autocomplete", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.AutocompleteProps{ComponentProps: i}.Layout(c, new(widget.Autocomplete))
		}},
		{"fileupload", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.FileUploadProps{ComponentProps: i}.Layout(c, new(widget.FileUpload))
		}},
		{"image", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.ImageProps{Width: 50, Height: 50, ComponentProps: i}.Layout(c, new(widget.Image))
		}},
		{"copy", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.CopyProps{Value: "x", ComponentProps: i}.Layout(c, new(widget.Copier))
		}},
		{"segmented", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.SegmentedProps{Options: []string{"a", "b"}, ComponentProps: i}.Layout(c, new(widget.Segments))
		}},
		{"sidebar", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.SidebarProps{ComponentProps: i}.Layout(c, new(widget.Sidebar))
		}},
		{"calendar", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.CalendarProps{Today: day, ComponentProps: i}.Layout(c, new(widget.Calendar))
		}},
		{"select", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.SelectProps{Options: []string{"a"}, ComponentProps: i}.Layout(c, new(widget.Select))
		}},
		{"number", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.NumberProps{ComponentProps: i}.Layout(c, new(widget.Stepper))
		}},
		{"figure", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.FigureProps{ComponentProps: i}.Layout(c, noop)
		}},
		{"highlight", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.HighlightProps{Text: "abc", Match: "b", ComponentProps: i}.Layout(c)
		}},
		{"feed", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.FeedProps{Count: 1, ComponentProps: i}.Layout(c, func(int) ayra.Widget { return noop })
		}},
		{"dialog", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			open := new(widget.Dialog)
			open.Open()
			return widget.DialogProps{ComponentProps: i}.Layout(c, open, noop)
		}},
		{"colourpicker", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.ColourPickerProps{ComponentProps: i}.Layout(c, new(widget.ColourPicker))
		}},
		{"datepicker", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.DatePickerProps{Today: day, ComponentProps: i}.Layout(c, new(widget.DatePicker))
		}},
		{"field", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.FieldProps{Label: "x", ComponentProps: i}.Layout(c, noop)
		}},
		{"password", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.PasswordProps{ComponentProps: i}.Layout(c, new(widget.Secret))
		}},
		{"filepicker", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.FilePickerProps{ComponentProps: i}.Layout(c, new(widget.FilePicker))
		}},
		{"collapsible", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.CollapsibleProps{Summary: "x", ComponentProps: i}.Layout(c, new(widget.Disclosure), noop)
		}},
		{"accordion", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.AccordionProps{Sections: []string{"a"}, ComponentProps: i}.Layout(c, new(widget.Accordion), func(int) ayra.Widget { return noop })
		}},
		{"item", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.ItemProps{Title: "x", ComponentProps: i}.Layout(c, new(widget.Button), nil)
		}},
		{"meter", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.MeterProps{Value: .5, ComponentProps: i}.Layout(c)
		}},
		{"buttongroup", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.ButtonGroupProps{Labels: []string{"a"}, ComponentProps: i}.Layout(c, new(widget.Group))
		}},
		{"menu", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.MenuProps{Entries: []string{"a"}, ComponentProps: i}.Layout(c, new(widget.Menu), noop)
		}},
		{"popover", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.PopoverProps{ComponentProps: i}.Layout(c, new(widget.Menu), noop, noop)
		}},
		{"menubar", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.MenubarProps{ComponentProps: i}.Layout(c, new(widget.Menubar))
		}},
		{"breadcrumb", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.BreadcrumbProps{Steps: []string{"a"}, ComponentProps: i}.Layout(c, new(widget.Crumbs))
		}},
		{"link", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.LinkProps{Text: "x", ComponentProps: i}.Layout(c, new(widget.Button))
		}},
		{"empty", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.EmptyProps{Title: "x", ComponentProps: i}.Layout(c)
		}},
		{"stat", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.StatProps{Label: "x", Value: "1", ComponentProps: i}.Layout(c)
		}},
		{"tooltip", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.TooltipProps{Text: "x", ComponentProps: i}.Layout(c, new(widget.Hover), noop)
		}},
		{"toast", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.ToastProps{ComponentProps: i}.Layout(c, new(widget.Toasts))
		}},
		{"pagination", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.PaginationProps{Total: 3, ComponentProps: i}.Layout(c, new(widget.Pages))
		}},
		{"drawer", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.DrawerProps{ComponentProps: i}.Layout(c, new(widget.Dialog), noop)
		}},
		{"split", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.SplitProps{ComponentProps: i}.Layout(c, new(widget.Split))
		}},
		{"slider", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.SliderProps{ComponentProps: i}.Layout(c, new(widget.Slider))
		}},
		{"rating", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.RatingProps{ComponentProps: i}.Layout(c, new(widget.Rating))
		}},
		{"palette", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.PaletteProps{ComponentProps: i}.Layout(c, new(widget.Palette))
		}},
		{"tabs", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.TabsProps{Labels: []string{"a"}, ComponentProps: i}.Layout(c, new(widget.Tabs))
		}},
		{"text", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.TextProps{Content: "x", ComponentProps: i}.Layout(c)
		}},
		{"checkbox", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.CheckboxProps{Label: "x", ComponentProps: i}.Layout(c, new(widget.Toggle))
		}},
		{"radio", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.RadioProps{Label: "x", ComponentProps: i}.Layout(c, new(widget.Toggle))
		}},
		{"switch", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.SwitchProps{Label: "x", ComponentProps: i}.Layout(c, new(widget.Toggle))
		}},
		{"carousel", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.CarouselProps{Count: 2, ComponentProps: i}.Layout(c, new(widget.Carousel), func(c ayra.Context, index int) ayra.Dimensions { return noop(c) })
		}},
		{"icon", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.IconProps{Icon: icons.Gear(), ComponentProps: i}.Layout(c)
		}},
		{"relativetime", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.RelativeTimeProps{At: day, Now: day.Add(time.Hour), ComponentProps: i}.Layout(c)
		}},
		{"loadmore", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.LoadMoreProps{ComponentProps: i}.Layout(c, new(widget.LoadMore))
		}},
		{"editinplace", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.EditInPlaceProps{Value: "x", ComponentProps: i}.Layout(c, new(widget.EditInPlace))
		}},
		{"inputgroup", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.InputGroupProps{ComponentProps: i}.Layout(c, noop)
		}},
		{"inputaddon", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.InputAddonProps{Text: ".com.br", ComponentProps: i}.Layout(c)
		}},
		{"themetoggle", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.ThemeToggleProps{ComponentProps: i}.Layout(c, new(widget.Button))
		}},
		{"output", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.OutputProps{Value: "R$ 10,00", ComponentProps: i}.Layout(c)
		}},
		{"responsiveimage", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.ResponsiveImageProps{Width: 50, Height: 50, ComponentProps: i}.Layout(c, new(widget.ResponsiveImage))
		}},
		{"dialogtrigger", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.DialogTriggerProps{Label: "x", ComponentProps: i}.Layout(c, new(widget.Button))
		}},
		{"deleterow", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.DeleteRowProps{ComponentProps: i}.Layout(c, new(widget.DeleteRow))
		}},
		{"lazyload", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.LazyLoadProps{Immediate: true, ComponentProps: i}.Layout(c, new(widget.LazyLoad), noop)
		}},
		{"optimistictoggle", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.OptimisticToggleProps{Label: "x", ComponentProps: i}.Layout(c, new(widget.OptimisticToggle))
		}},
		{"activesearch", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.ActiveSearchProps{ComponentProps: i}.Layout(c, new(widget.ActiveSearch))
		}},
		{"hovercard", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.HoverCardProps{ComponentProps: i}.Layout(c, new(widget.HoverCard), noop, noop)
		}},
		{"formdialog", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			open := new(widget.FormDialog)
			open.Open()
			return widget.FormDialogProps{Title: "x", ComponentProps: i}.Layout(c, open)
		}},
		{"tree", func(c ayra.Context, i widget.ComponentProps) ayra.Dimensions {
			return widget.TreeProps{ComponentProps: i}.Layout(c, new(widget.Tree))
		}},
	}
}

// shapes are the widgets whose root is the shape itself, drawn at a size the
// caller fixed: a bar, a disc, a rule. Padding has no meaning on them -- what
// the root restyles is the fill and the corners -- so the room they take is not
// what shows the override, and the source test that every published part is
// resolved is the guard for them.
var shapes = map[string]bool{"progress": true, "avatar": true, "skeleton": true, "separator": true, "image": true, "video": true, "responsiveimage": true}

func TestEveryComponentTakesAnOverride(t *testing.T) {
	for _, w := range everyWidget() {
		if shapes[w.name] {
			continue
		}
		t.Run(w.name, func(t *testing.T) {
			ctx, _ := frame(t, theme.Light, 400)
			plain := w.draw(ctx, widget.ComponentProps{}).Size

			ctx, _ = frame(t, theme.Light, 400)
			styled := w.draw(ctx, restyle).Size

			if styled.X <= plain.X && styled.Y <= plain.Y {
				t.Errorf("%s took %v with a padding of 30 on its root and %v without: the widget embeds ComponentProps and ignores it", w.name, styled, plain)
			}
		})
	}
}

// TestTheThemeRestylesEveryComponentToo is the other place an override comes from.
func TestTheThemeRestylesEveryComponentToo(t *testing.T) {
	for _, w := range everyWidget() {
		if shapes[w.name] {
			continue
		}
		t.Run(w.name, func(t *testing.T) {
			ctx, _ := frame(t, theme.Light, 400)
			plain := w.draw(ctx, widget.ComponentProps{}).Size

			ctx, _ = frame(t, theme.Light, 400)
			ctx.Theme.Styles = &theme.Styles{Look: theme.Look{w.name: {"root": {Padding: theme.Uniform(30)}}}}
			styled := w.draw(ctx, widget.ComponentProps{}).Size

			if styled.X <= plain.X && styled.Y <= plain.Y {
				t.Errorf("%s ignored the theme's look for it", w.name)
			}
		})
	}
}

// notYet are the widgets that do not take ComponentProps, each with the reason.
// A widget on this list is a promise not yet kept, in the open: ADR-0131's third
// test exists so that a list is the directory, and a widget that is on neither
// side is a failure.
var notYet = map[string]string{
	"table":   "the listing is being rebuilt on the datatable-native branch, and takes its parts there",
	"toolbar": "lives in table.go, with the listing",
	"player":  "the transport has not taken its parts yet; Video, the picture it sits under, has",
}

// TestEveryExtensibleComponentIsInThisTable is the third of ADR-0131's tests: it
// reads the directory. The first two walk a list written by hand, and this is
// what makes the list the tree -- a Props type that embeds ComponentProps and has
// no row above fails, and so does one that embeds nothing and is not declared
// above as not yet.
func TestEveryExtensibleComponentIsInThisTable(t *testing.T) {
	declaration := regexp.MustCompile(`(?ms)^type ([A-Z][A-Za-z0-9]*)Props struct \{\n(.*?)^\}`)

	inTable := map[string]bool{}
	for _, w := range everyWidget() {
		inTable[w.name] = true
	}

	files, err := filepath.Glob("*.go")
	if err != nil || len(files) < 20 {
		t.Fatalf("the directory was not read: %v, %d files", err, len(files))
	}
	seen := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, found := range declaration.FindAllStringSubmatch(string(body), -1) {
			seen++
			name := strings.ToLower(found[1])
			embeds := strings.Contains(found[2], "ComponentProps")

			switch {
			case embeds && !inTable[name]:
				t.Errorf("%sProps embeds ComponentProps and has no row in everyWidget: nothing proves it takes an override", found[1])
			case !embeds && notYet[name] == "":
				t.Errorf("%sProps does not embed ComponentProps and is not declared in notYet with a reason", found[1])
			case embeds && notYet[name] != "":
				t.Errorf("%sProps is declared not yet and takes ComponentProps: the declaration no longer describes anything", found[1])
			}
		}
	}
	if seen < 60 {
		t.Errorf("only %d Props types were read, and there are many more", seen)
	}
}
