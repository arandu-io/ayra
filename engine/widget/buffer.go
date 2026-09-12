package widget

import (
	"errors"
	"io"
	"unicode/utf8"
)

// editBuffer stores the bytes before and after the caret around unused space.
type editBuffer struct {
	text             []byte
	gapstart, gapend int
	changed          bool
}

var _ textSource = (*editBuffer)(nil)

const minGapSize = 5

var errNegativeBufferOffset = errors.New("negative buffer offset")

func (e *editBuffer) Changed() bool {
	changed := e.changed
	e.changed = false
	return changed
}

func (e *editBuffer) Size() int64 {
	return int64(len(e.text) - e.gapLen())
}

func (e *editBuffer) gapLen() int {
	return e.gapend - e.gapstart
}

// moveGap places the unused region at caret and reserves at least space bytes.
func (e *editBuffer) moveGap(caret, space int) {
	size := int(e.Size())
	caret = min(max(caret, 0), size)

	switch {
	case caret < e.gapstart:
		moved := e.gapstart - caret
		copy(e.text[e.gapend-moved:e.gapend], e.text[caret:e.gapstart])
		e.gapstart = caret
		e.gapend -= moved
	case caret > e.gapstart:
		moved := caret - e.gapstart
		copy(e.text[e.gapstart:e.gapstart+moved], e.text[e.gapend:e.gapend+moved])
		e.gapstart += moved
		e.gapend += moved
	}

	if e.gapLen() >= space {
		return
	}

	gapLen := max(space, minGapSize)
	grown := make([]byte, size+gapLen)
	copy(grown[:caret], e.text[:e.gapstart])
	copy(grown[caret+gapLen:], e.text[e.gapend:])
	e.text = grown
	e.gapstart = caret
	e.gapend = caret + gapLen
}

// deleteRunes widens the gap over runes beside caret. A negative count walks
// backwards, which also moves the insertion point to the beginning of what was
// removed.
func (e *editBuffer) deleteRunes(caret int, count int64) {
	e.moveGap(caret, 0)
	before := e.gapLen()

	for count < 0 && e.gapstart > 0 {
		_, size := utf8.DecodeLastRune(e.text[:e.gapstart])
		e.gapstart -= size
		count++
	}
	for count > 0 && e.gapend < len(e.text) {
		_, size := utf8.DecodeRune(e.text[e.gapend:])
		e.gapend += size
		count--
	}

	e.changed = e.changed || e.gapLen() != before
}

func (e *editBuffer) ReadAt(dst []byte, offset int64) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}
	if offset < 0 {
		return 0, errNegativeBufferOffset
	}
	if offset >= e.Size() {
		return 0, io.EOF
	}

	requested := len(dst)
	logical := int(offset)
	read := 0
	if logical < e.gapstart {
		n := copy(dst, e.text[logical:e.gapstart])
		dst = dst[n:]
		read += n
		logical += n
	}
	if len(dst) > 0 && logical >= e.gapstart {
		read += copy(dst, e.text[logical+e.gapLen():])
	}
	if read < requested {
		return read, io.EOF
	}
	return read, nil
}

func (e *editBuffer) ReplaceRunes(byteOffset, runeCount int64, replacement string) {
	byteOffset = min(max(byteOffset, 0), e.Size())
	replacement = string([]rune(replacement))

	e.deleteRunes(int(byteOffset), runeCount)
	insertion := e.gapstart
	e.moveGap(insertion, len(replacement))
	copy(e.text[e.gapstart:], replacement)
	e.gapstart += len(replacement)
	e.changed = e.changed || len(replacement) > 0
}
