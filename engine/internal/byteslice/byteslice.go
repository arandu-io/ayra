// Package byteslice reads the memory of a Go value as bytes, without copying
// it.
//
// It is here for the one step where a typed value has to leave Go. A graphics
// driver is handed an address and a count of bytes; it has no notion of a
// []float32 or of a struct of uniforms, and the only thing it can be given is
// the memory those already occupy. Copying each of them into a fresh []byte on
// the way out would be a second copy of every buffer, on every frame, for data
// measured in megabytes -- so this is unsafe deliberately, in one small place,
// instead of accidentally in twenty.
//
// Everything here returns an alias, never a copy: the view and the value it was
// taken from are one piece of memory under two names. That is what the caller
// is buying, and it is what the caller has to keep to.
//
//   - The value has to outlive the view. Nothing inside a []byte tells the
//     collector that the slice or the struct it points into is still in use, so
//     a value whose last reference is the view taken from it can be collected
//     while that view is still being read. Keep the original reachable for as
//     long as the view is.
//   - The value must not move. Appending to the slice behind a view, or
//     reassigning it, leaves the view describing the allocation that was
//     abandoned, and it goes on reading it with nothing to say that the memory
//     now belongs to somebody else.
//   - Writing through a view writes the original. That is the point of not
//     copying, and it is equally the way to overwrite a value something else is
//     reading at that moment.
//
// A view of nothing is nothing: a nil slice, an empty slice and a type of no
// width each give an empty view rather than a panic or a byte that is not
// there.
package byteslice

import "unsafe"

// View returns the bytes of the value v points at, aliasing it rather than
// copying it.
//
// The view is exactly as wide as the value, padding included, because padding
// is memory the value occupies and a driver reads it along with the rest. A
// type of no width gives an empty view.
//
// v must not be nil. The runtime stops the program here rather than hand back a
// view of address zero, which is a failure that would otherwise be found by
// whatever read the view instead of by whoever built it -- the exception being
// a type of no width, where there is nothing to read and the view is empty. The
// value must stay alive and in place for as long as the view is used -- see the
// package comment.
func View[T any](v *T) []byte {
	// The width is taken from a declared value rather than from *v, so that
	// nothing in this expression reads as a dereference of a pointer that has
	// not been checked. Only the type is being measured either way.
	var zero T
	return unsafe.Slice((*byte)(unsafe.Pointer(v)), unsafe.Sizeof(zero))
}

// Slice returns the bytes of the elements of s, aliasing them rather than
// copying them.
//
// The view begins at the first element and is len(s) elements wide -- the count
// times the width of one, padding included -- which is the address and the
// length a driver is handed. Its capacity spans the capacity of s rather than
// its length, so the view describes the whole allocation and an appending
// caller writes into room the slice already owns.
//
// A nil slice, an empty one and a slice of a type of no width all give an empty
// view. There is no first element to take the address of in those cases, and
// the answer is nothing rather than a panic.
//
// The elements must stay alive and in place for as long as the view is used --
// see the package comment. Appending to s is the case that goes wrong without a
// sign: the append may move the elements elsewhere, and the view is then a
// reader of the allocation that s has left behind.
func Slice[T any](s []T) []byte {
	// As in View: the width is the type's, and asking for it through s[0] would
	// read as an access to an element that an empty slice does not have.
	var zero T
	width := unsafe.Sizeof(zero)

	if cap(s) == 0 {
		// No room means no first element, and what its address would be is
		// deliberately left unspecified by the language. The empty view is
		// built here rather than out of an address nobody defined.
		return nil
	}

	// Taken over the capacity and then cut to the length, so that the bytes
	// beyond len(s) stay reachable as capacity instead of being lost, exactly
	// as they are in s.
	all := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(s))), width*uintptr(cap(s)))
	return all[:width*uintptr(len(s))]
}
