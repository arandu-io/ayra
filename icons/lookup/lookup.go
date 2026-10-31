// Package lookup finds an icon by its name, for an application whose icons come
// as data: a menu a server describes, an entry a configuration names.
//
// It is a package of its own for the reason package icons has one function per
// icon. A map from a name to an icon holds every icon, because the linker cannot
// know which names will be asked for, and importing this package is the choice to
// carry the whole set -- about 600 KB of path data. An application that draws
// thirty icons it names in code imports icons and never this.
package lookup

import (
	"sort"

	"github.com/arandu-io/ayra/icons"
)

// Icon answers the icon with this name: the name of its Go function, "Trash", or
// Phosphor's own, "trash". The second result is false for a name the set does
// not have, and the icon is then the zero one, which draws nothing.
func Icon(name string) (icons.Icon, bool) {
	if f, found := all[name]; found {
		return f(), true
	}
	return icons.Icon{}, false
}

// Names answers every name the set answers to, sorted.
func Names() []string {
	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
