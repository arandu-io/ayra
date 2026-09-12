package text

import (
	"strconv"
	"testing"
)

// held reports which of the given keys the cache still answers, in order,
// without disturbing the recency order of the ones it finds.
//
// Reading is what promotes an entry, so a helper that reported the contents by
// reading them would rewrite the very order the test is about. It is used only
// where the test is finished with the ordering, or where every key is looked at
// once and the resulting order is spelled out by the caller.
func held(l *lru[string, int], keys ...string) []string {
	var found []string
	for _, k := range keys {
		if _, ok := l.Get(k); ok {
			found = append(found, k)
		}
	}
	return found
}

// A read moves an entry back to the front.
//
// This is the whole difference between this and a queue. Without it the cache
// throws away what is being used most and keeps what was merely written last,
// which is worse than no cache: it pays for the bookkeeping and still misses.
func TestAReadMovesAnEntryBackToTheFront(t *testing.T) {
	l := &lru[string, int]{capacity: 3}
	l.Put("a", 1)
	l.Put("b", 2)
	l.Put("c", 3)

	// "a" is the oldest by insertion. Reading it makes it the newest, so the
	// next insertion has to take "b" instead.
	if _, ok := l.Get("a"); !ok {
		t.Fatal("a was not held")
	}
	l.Put("d", 4)

	if _, ok := l.Get("b"); ok {
		t.Error("b survived, and it was the least recently used")
	}
	for _, k := range []string{"a", "c", "d"} {
		if _, ok := l.Get(k); !ok {
			t.Errorf("%s was evicted, and it was not the least recently used", k)
		}
	}
}

// A full cache evicts the least recently used entry, and not another one.
//
// Stated positively in the assertion below, because "something was evicted" is
// the easy half and it is also what a broken cache satisfies. What matters is
// which one went.
func TestAFullCacheEvictsTheLeastRecentlyUsedAndNotAnother(t *testing.T) {
	l := &lru[string, int]{capacity: 4}
	for _, k := range []string{"a", "b", "c", "d"} {
		l.Put(k, 0)
	}

	// Touch them in an order that has nothing to do with how they went in, so
	// that insertion order cannot be mistaken for recency.
	for _, k := range []string{"c", "a", "d", "b"} {
		if _, ok := l.Get(k); !ok {
			t.Fatalf("%s was evicted before the cache was full", k)
		}
	}

	// "c" is now the least recently used, by reads alone.
	l.Put("e", 0)

	if _, ok := l.Get("c"); ok {
		t.Error("c survived, and it was the least recently read")
	}
	if got, want := held(l, "a", "b", "d", "e"), []string{"a", "b", "d", "e"}; len(got) != len(want) {
		t.Errorf("the cache holds %q and %q was expected", got, want)
	}
}

// Writing the same key twice replaces the value and does not grow the cache.
//
// The second write is an update, not a second entry. A cache that appended
// instead would keep a dead copy of every key ever overwritten -- invisible,
// because the map still answers one value per key, and fatal at the moment of
// eviction: the oldest thing in the list would be a dead copy, and throwing it
// away would take the live entry of a key that is in active use with it.
func TestWritingTheSameKeyTwiceDoesNotGrowTheCache(t *testing.T) {
	l := &lru[string, int]{capacity: 2}

	for i := range 50 {
		l.Put("a", i)
	}
	entries := 0
	for e := l.tail.next; e != l.head; e = e.next {
		entries++
		if entries > 50 {
			t.Fatal("the recency list did not close after the rewritten entries")
		}
	}
	if entries != 1 {
		t.Fatalf("rewriting one key left %d entries in the recency list", entries)
	}

	if v, ok := l.Get("a"); !ok || v != 49 {
		t.Fatalf("a is %d, %v and the last value written was 49", v, ok)
	}

	// One more key than the rewritten one, well inside a capacity of two. If
	// the rewrites piled up, this insertion has a queue of dead copies of "a"
	// in front of it and evicting the oldest takes "a" itself.
	l.Put("b", 0)
	if _, ok := l.Get("a"); !ok {
		t.Error("a was evicted by a single insertion into a cache of capacity two")
	}
	if _, ok := l.Get("b"); !ok {
		t.Error("b was not held")
	}
}

// A rewritten key is as recent as any other write.
//
// Put is a use. A cache that replaced the value without moving the entry would
// keep a key that is being written every frame at the back of the queue and
// evict it while it is hot.
func TestARewrittenKeyIsAsRecentAsAnyOtherWrite(t *testing.T) {
	l := &lru[string, int]{capacity: 2}
	l.Put("a", 1)
	l.Put("b", 2)
	l.Put("a", 3)

	// "b" is now the least recently used: "a" was written after it.
	l.Put("c", 4)

	if _, ok := l.Get("b"); ok {
		t.Error("b survived, and it was the least recently used")
	}
	if v, ok := l.Get("a"); !ok || v != 3 {
		t.Errorf("a is %d, %v and it was rewritten to 3 after b", v, ok)
	}
}

// The entry just inserted survives the next insertion whenever there is room
// for two.
//
// It is the one thing a cache must never get wrong. An implementation that
// evicted the newest -- a list walked from the wrong end -- passes every test
// that only counts entries, and in use it answers a miss for the one key the
// caller is certain to ask for next.
func TestTheNewestEntrySurvivesTheNextInsertion(t *testing.T) {
	for capacity := 2; capacity <= 8; capacity++ {
		t.Run(strconv.Itoa(capacity), func(t *testing.T) {
			l := &lru[string, int]{capacity: capacity}
			for i := range capacity * 4 {
				k := strconv.Itoa(i)
				l.Put(k, i)
				if _, ok := l.m[k]; !ok {
					t.Fatalf("the key written last was gone at insertion %d", i)
				}
				if i > 0 {
					prev := strconv.Itoa(i - 1)
					if _, ok := l.m[prev]; !ok {
						t.Fatalf("key %s did not survive the insertion after it", prev)
					}
				}
			}
		})
	}
}

// A capacity of one holds exactly the last key written.
//
// The smallest cache that is still a cache, and the one where the list is a
// single entry that has to be removed and reinserted correctly. Off by one
// here is either a cache that holds nothing or one that never evicts.
func TestACapacityOfOneHoldsTheLastKeyWritten(t *testing.T) {
	l := &lru[string, int]{capacity: 1}

	l.Put("a", 1)
	l.Put("b", 2)

	if _, ok := l.Get("a"); ok {
		t.Error("a survived a cache of capacity one")
	}
	if v, ok := l.Get("b"); !ok || v != 2 {
		t.Errorf("b is %d, %v and it was the last key written", v, ok)
	}
}

// A cache with no capacity set uses the package default, and the zero value is
// ready to use.
//
// Every cache in the shaper is a zero-valued field, never constructed, so an
// unset capacity has to mean the default rather than nothing. Read literally as
// zero it would be a cache that holds no entry at all: correct in every answer
// it gives and a shaper that reshapes every glyph of every frame.
func TestACacheWithNoCapacitySetUsesTheDefault(t *testing.T) {
	var l lru[string, int]

	for i := range maxSize {
		l.Put(strconv.Itoa(i), i)
	}
	for i := range maxSize {
		if _, ok := l.Get(strconv.Itoa(i)); !ok {
			t.Fatalf("key %d was evicted before the default capacity was reached", i)
		}
	}

	// The reads above ran from 0 upwards, so 0 is the least recently used again.
	l.Put(strconv.Itoa(maxSize), maxSize)
	if _, ok := l.Get("0"); ok {
		t.Error("the cache grew past the default capacity")
	}
	if _, ok := l.Get(strconv.Itoa(maxSize)); !ok {
		t.Error("the key that caused the eviction was the one evicted")
	}
}

// A read of a key that was never written answers nothing, on a cache that was
// never written to at all.
//
// The zero value has no map and no list, and this is the path that reaches it
// first: a shaper that draws before it caches anything asks here.
func TestAReadOfAnEmptyCacheAnswersNothing(t *testing.T) {
	var l lru[string, int]

	if v, ok := l.Get("a"); ok || v != 0 {
		t.Errorf("an empty cache answered %d, %v", v, ok)
	}
}

// The glyph cache answers only when the glyphs are the ones it stored.
//
// Its key is a hash, and a hash has collisions. The stored glyphs are compared
// before the value is handed back, so a collision is a miss and never the
// wrong shaped run drawn under another string's glyphs.
func TestTheGlyphCacheAnswersOnlyForTheGlyphsItStored(t *testing.T) {
	var c glyphLRU[int]

	stored := []Glyph{{ID: 1}, {ID: 2}}
	c.Put(7, stored, 42)

	if v, ok := c.Get(7, stored); !ok || v != 42 {
		t.Errorf("the stored glyphs answered %d, %v and 42 was expected", v, ok)
	}
	if _, ok := c.Get(7, []Glyph{{ID: 1}, {ID: 3}}); ok {
		t.Error("a collision on the key answered another string's value")
	}
	if _, ok := c.Get(7, []Glyph{{ID: 1}}); ok {
		t.Error("a prefix of the stored glyphs answered the stored value")
	}
}
