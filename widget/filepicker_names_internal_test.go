package widget

import "testing"

// TestAPickerNeverHandsBackANameThatIsNotOne keeps a listing from the server
// from naming a place outside the directory it lists.
//
// The picker hands back an entry's own name and the caller joins it to the
// path it is in. A name with a separator in it, a NUL, or one that is "." or
// ".." is not the name of one entry in one directory: joined, it is a path
// somewhere else -- "../../.ssh/id_ed25519" chosen as though it were a file in
// the folder on the screen. Such an entry is not drawn, so it can be neither
// chosen nor opened.
func TestAPickerNeverHandsBackANameThatIsNotOne(t *testing.T) {
	refused := []string{"", ".", "..", "../secret", "a/b", `a\b`, "/etc", "x\x00y"}
	entries := []FileEntry{{Name: "report.pdf"}, {Name: "photos", Dir: true}}
	for _, name := range refused {
		entries = append(entries, FileEntry{Name: name}, FileEntry{Name: name, Dir: true})
	}
	props := FilePickerProps{Entries: entries}

	for _, entry := range props.visible("") {
		for _, name := range refused {
			if entry.Name == name {
				t.Errorf("the listing draws an entry named %q", name)
			}
		}
	}
	if got := len(props.visible("")); got != 2 {
		t.Errorf("the listing draws %d entries, want the two that are names", got)
	}

	// Every row pressed, the way a person working down the list would.
	var state FilePicker
	props.Layout(carouselFrame(t, 400), &state)
	for index := range state.rows {
		state.rows[index].click.Click()
		props.Layout(carouselFrame(t, 400), &state)
		for _, report := range []func() (string, bool){state.Chosen, state.Opened} {
			if name, ok := report(); ok && name != "report.pdf" && name != "photos" {
				t.Errorf("the picker handed back %q", name)
			}
		}
	}
}
