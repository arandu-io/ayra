package input

import (
	"image"
	"io"
	"slices"

	"github.com/arandu-io/ayra/engine/f32"
	f32internal "github.com/arandu-io/ayra/engine/internal/f32"
	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/io/semantic"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/io/transfer"
)

// This file answers one question, over and over: given a point on the screen,
// which controls does it belong to, and in what order.
//
// The answer is rebuilt from scratch on every frame. A frame declares its
// areas and its handlers as it draws them, in drawing order, and what is left
// at the end is a tree of shapes with handlers hanging off it. Nothing carries
// over: a control that moved is gone from where it was, because on a screen
// that scrolls, that is every control on every frame.
//
// The shape matters, not just the extent. An area clipped to an ellipse is hit
// as an ellipse, and a hit test that settled for the enclosing rectangle would
// make a round control answer presses in its corners -- outside anything that
// was ever painted, and invisible in every picture of the screen, because the
// part that is wrong is the part that is not drawn.

// pointerQueue is the hit tree for one frame, and the semantic tree that hangs
// off the same areas.
//
// It is rebuilt every frame by a pointerCollector and read by everything that
// asks a question about a point: event routing, system actions, accessibility.
type pointerQueue struct {
	// hitTree is the frame's handlers and areas in declaration order, which is
	// drawing order. It is walked backwards, so the last thing declared -- the
	// thing drawn on top -- is the first thing asked.
	hitTree []hitNode
	// areas is the shapes themselves, each pointing at its parent, so that a
	// hit has to survive every clip it was declared inside.
	areas []areaNode

	semantic struct {
		idsAssigned bool
		lastID      SemanticID
		// contentIDs maps semantic content to the ids previously handed out
		// for it. Assistive technology follows a control by its id, so an id
		// that changed every frame would be a new control every frame: the
		// same content gets the same id back for as long as it keeps being
		// declared.
		contentIDs map[semanticContent][]semanticID
	}
}

// hitNode is one entry in the frame's declaration order: either an area being
// entered, or a handler registering inside the area currently open.
type hitNode struct {
	// next is the index of the node enclosing this one, or -1. A node that
	// stops the walk hands it back to this index, which is how the walk skips
	// everything drawn underneath a control and still reaches the controls
	// that contain it.
	next int
	// area is the shape this node is tested against.
	area int

	// tag is the handler, for handler nodes. It is nil on area nodes.
	tag event.Tag
	// pass says the node lets the walk carry on past it to whatever is
	// underneath. Areas always do; a handler does only when it was declared
	// inside a pass region.
	pass bool
}

// pointerState is what the router remembers about pointers between events.
//
// It is a value, copied and replaced rather than mutated, so that the state
// belonging to a frame stays the state that frame was delivered.
type pointerState struct {
	cursor   pointer.Cursor
	pointers []pointerInfo
}

// pointerInfo tracks one pointer -- one mouse, or one finger -- from the moment
// it is first seen until it is neither pressed nor over anything.
type pointerInfo struct {
	id      pointer.ID
	pressed bool
	// handlers is who this pointer reports to. While the pointer is pressed
	// the set is frozen: a press belongs to what it went down on, and a drag
	// that stopped reporting when it left would make every slider end at its
	// own edge.
	handlers []event.Tag
	// last is the last event received for this pointer, replayed when a frame
	// changes what is under it without the pointer having moved.
	last pointer.Event

	// entered is who currently believes the pointer is inside them, which is
	// what the next enter and leave events are computed against.
	entered []event.Tag

	// dataSource is the handler a drag of transferable data started from, and
	// dataTarget the one it is being offered to.
	dataSource event.Tag
	dataTarget event.Tag
}

// pointerHandler is the pointer half of one handler's state, kept across
// frames by the router.
type pointerHandler struct {
	// areaPlusOne is the index into pointerQueue.areas, plus one, so that the
	// zero value means "declared no area this frame" rather than "area 0".
	areaPlusOne int
	// setup records that the handler has already been told, with a cancel
	// event, that whatever it thought was in progress is not. A handler that
	// never got that would start life believing in a press nobody made.
	setup bool
}

// pointerFilter is every pointer filter one handler declared, merged into one.
//
// Merged rather than kept as a list because the questions asked of it -- does
// this event match, how much of this scroll is accepted -- have one answer for
// the handler, and a list would compute it by walking the list every time.
type pointerFilter struct {
	kinds pointer.Kind
	// scrollX and scrollY are how far the handler is willing to be scrolled in
	// each direction. Scroll past them is left for whatever is underneath.
	scrollX, scrollY pointer.ScrollRange

	sourceMimes []string
	targetMimes []string
}

// areaOp is a shape a point is tested against: the extent, and which shape the
// extent describes.
type areaOp struct {
	kind areaKind
	rect image.Rectangle
}

// areaNode is one shape in the frame's tree of clips, with whatever was
// attached to it while it was open.
type areaNode struct {
	trans f32.Affine2D
	area  areaOp

	cursor pointer.Cursor

	// Tree indices, with -1 being the sentinel.
	parent     int
	firstChild int
	lastChild  int
	sibling    int

	semantic struct {
		valid   bool
		id      SemanticID
		content semanticContent
	}
	action system.Action
}

// areaKind is which shape an area's rectangle describes.
type areaKind uint8

const (
	// areaRect is the rectangle itself.
	areaRect areaKind = iota
	// areaEllipse is the ellipse inscribed in it.
	areaEllipse
)

// collectState is the part of a pointerCollector that a clip saves and
// restores: where in the tree it is, and what transform is in effect.
type collectState struct {
	t f32.Affine2D
	// nodePlusOne is the current node index, plus one, so that the zero value
	// is the initial state rather than node 0.
	nodePlusOne int
	// pass counts the pass regions currently open. Handlers declared while it
	// is above zero do not stop the walk.
	pass int
}

// pointerCollector builds a pointerQueue from the operations of one frame.
type pointerCollector struct {
	q     *pointerQueue
	state collectState
	// nodeStack is the enclosing node of each area still open, so that popping
	// an area returns to where it was pushed.
	nodeStack []int
}

// semanticContent is everything said about a control, and it is the key its
// semantic id is looked up by. Two controls described identically are, to
// anything reading the tree, the same control -- which is why the ids are a
// list per content and not one id.
type semanticContent struct {
	tag      event.Tag
	label    string
	desc     string
	class    semantic.ClassOp
	gestures SemanticGestures
	selected bool
	disabled bool
}

// semanticID is one assigned id and whether this frame has claimed it.
type semanticID struct {
	id   SemanticID
	used bool
}

// Reset starts a new frame: the previous frame's tree is dropped and the
// implicit root put back.
func (c *pointerCollector) Reset() {
	c.q.reset()
	c.resetState()
	c.ensureRoot()
}

func (c *pointerCollector) resetState() {
	c.state = collectState{
		t: f32.AffineId(),
	}
	c.nodeStack = c.nodeStack[:0]
	// Pop every node except the root.
	if len(c.q.hitTree) > 0 {
		c.state.nodePlusOne = 0 + 1
	}
}

// ensureRoot puts an area around the whole frame.
//
// Semantic descriptions hang off areas, and a frame that described something
// before clipping anything would have nowhere to hang it. The root is
// deliberately enormous rather than the window's size, because it is not a
// clip: it is the thing every other area is a child of.
func (c *pointerCollector) ensureRoot() {
	if len(c.q.areas) > 0 {
		return
	}
	c.pushArea(areaRect, image.Rect(-1e6, -1e6, 1e6, 1e6))
	// Make it semantic to ensure a single semantic root.
	c.q.areas[0].semantic.valid = true
}

func (c *pointerCollector) setTrans(t f32.Affine2D) {
	c.state.t = t
}

// clip opens an area for a clip operation.
//
// The shape the clip carries is the shape the area is hit against. This is the
// line between what a control looks like and what it catches, and there is no
// second place to keep them in step: if the hit test took the bounds and
// ignored the shape, a round control would be pressable in its corners and
// every picture of the screen would still be correct.
func (c *pointerCollector) clip(op ops.ClipOp) {
	kind := areaRect
	if op.Shape == ops.Ellipse {
		kind = areaEllipse
	}
	c.pushArea(kind, op.Bounds)
}

// pushArea opens an area as a child of whatever area is currently open.
func (c *pointerCollector) pushArea(kind areaKind, bounds image.Rectangle) {
	parentID := c.currentArea()
	areaID := len(c.q.areas)
	areaOp := areaOp{kind: kind, rect: bounds}
	if parentID != -1 {
		parent := &c.q.areas[parentID]
		if parent.firstChild == -1 {
			parent.firstChild = areaID
		}
		if siblingID := parent.lastChild; siblingID != -1 {
			c.q.areas[siblingID].sibling = areaID
		}
		parent.lastChild = areaID
	}
	an := areaNode{
		trans:      c.state.t,
		area:       areaOp,
		parent:     parentID,
		sibling:    -1,
		firstChild: -1,
		lastChild:  -1,
	}

	c.q.areas = append(c.q.areas, an)
	c.nodeStack = append(c.nodeStack, c.state.nodePlusOne-1)
	// An area itself never stops the walk: it narrows where the walk may go,
	// and what stops it is a handler.
	c.addHitNode(hitNode{
		area: areaID,
		pass: true,
	})
}

func (c *pointerCollector) popArea() {
	n := len(c.nodeStack)
	c.state.nodePlusOne = c.nodeStack[n-1] + 1
	c.nodeStack = c.nodeStack[:n-1]
}

// pass opens a region whose handlers let events through to what is underneath.
func (c *pointerCollector) pass() {
	c.state.pass++
}

func (c *pointerCollector) popPass() {
	c.state.pass--
}

func (c *pointerCollector) currentArea() int {
	if i := c.state.nodePlusOne - 1; i != -1 {
		n := c.q.hitTree[i]
		return n.area
	}
	return -1
}

func (c *pointerCollector) currentAreaBounds() image.Rectangle {
	a := c.currentArea()
	if a == -1 {
		panic("no root area")
	}
	return c.q.areas[a].bounds()
}

func (c *pointerCollector) addHitNode(n hitNode) {
	n.next = c.state.nodePlusOne - 1
	c.q.hitTree = append(c.q.hitTree, n)
	c.state.nodePlusOne = len(c.q.hitTree) - 1 + 1
}

// newHandler registers tag against the area currently open.
func (c *pointerCollector) newHandler(tag event.Tag, state *pointerHandler) {
	areaID := c.currentArea()
	c.addHitNode(hitNode{
		area: areaID,
		tag:  tag,
		pass: c.state.pass > 0,
	})
	state.areaPlusOne = areaID + 1
}

// inputOp registers a handler and names the area after it, so that what the
// accessibility tree reports about the area is reported about the control.
func (c *pointerCollector) inputOp(tag event.Tag, state *pointerHandler) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.content.tag = tag
	c.newHandler(tag, state)
}

// actionInputOp marks the current area as one the system may act on -- a drag
// that moves the window, for instance -- without a handler being involved.
func (c *pointerCollector) actionInputOp(act system.Action) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.action = act
}

func (c *pointerCollector) semanticLabel(lbl string) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.valid = true
	area.semantic.content.label = lbl
}

func (c *pointerCollector) semanticDesc(desc string) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.valid = true
	area.semantic.content.desc = desc
}

func (c *pointerCollector) semanticClass(class semantic.ClassOp) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.valid = true
	area.semantic.content.class = class
}

func (c *pointerCollector) semanticSelected(selected bool) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.valid = true
	area.semantic.content.selected = selected
}

func (c *pointerCollector) semanticEnabled(enabled bool) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.valid = true
	area.semantic.content.disabled = !enabled
}

func (c *pointerCollector) cursor(cursor pointer.Cursor) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.cursor = cursor
}

// Reset forgets the area a handler declared, which is what happens to a
// handler whose frame did not declare it.
func (s *pointerHandler) Reset() {
	s.areaPlusOne = 0
}

// ResetEvent returns the cancel event a handler is owed once, before it is
// told anything else.
//
// Without it a handler that outlived a frame it was mid-gesture in would keep
// believing in a press that no longer exists: a button left lit, a drag with
// no finger on it.
func (s *pointerHandler) ResetEvent() (event.Event, bool) {
	if s.setup {
		return nil, false
	}
	s.setup = true
	return pointer.Event{Kind: pointer.Cancel}, true
}

// Add merges one declared filter into the union.
func (p *pointerFilter) Add(f event.Filter) {
	switch f := f.(type) {
	case transfer.SourceFilter:
		if slices.Contains(p.sourceMimes, f.Type) {
			return
		}
		p.sourceMimes = append(p.sourceMimes, f.Type)
	case transfer.TargetFilter:
		if slices.Contains(p.targetMimes, f.Type) {
			return
		}
		p.targetMimes = append(p.targetMimes, f.Type)
	case pointer.Filter:
		p.kinds = p.kinds | f.Kinds
		p.scrollX = p.scrollX.Union(f.ScrollX)
		p.scrollY = p.scrollY.Union(f.ScrollY)
	}
}

// Matches reports whether the handler asked for this event.
func (p *pointerFilter) Matches(e event.Event) bool {
	switch e := e.(type) {
	case pointer.Event:
		return e.Kind&p.kinds == e.Kind
	case transfer.CancelEvent, transfer.InitiateEvent:
		return len(p.sourceMimes) > 0 || len(p.targetMimes) > 0
	case transfer.RequestEvent:
		if slices.Contains(p.sourceMimes, e.Type) {
			return true
		}
	case transfer.DataEvent:
		if slices.Contains(p.targetMimes, e.Type) {
			return true
		}
	}
	return false
}

// Merge folds another union into this one.
func (p *pointerFilter) Merge(p2 pointerFilter) {
	p.kinds = p.kinds | p2.kinds
	p.scrollX = p.scrollX.Union(p2.scrollX)
	p.scrollY = p.scrollY.Union(p2.scrollY)
	p.sourceMimes = append(p.sourceMimes, p2.sourceMimes...)
	p.targetMimes = append(p.targetMimes, p2.targetMimes...)
}

// clampScroll splits a scroll distance into the part this filter accepts and
// the part left over.
//
// Split rather than clipped, because the remainder is what makes a list inside
// a list behave: the inner one takes what it can move by and the outer one
// takes the rest, so that scrolling past the end of the inner list carries on
// scrolling the page instead of stopping dead.
func (p *pointerFilter) clampScroll(scroll f32.Point) (left, scrolled f32.Point) {
	left.X, scrolled.X = clampSplit(scroll.X, p.scrollX.Min, p.scrollX.Max)
	left.Y, scrolled.Y = clampSplit(scroll.Y, p.scrollY.Min, p.scrollY.Max)
	return
}

func clampSplit(v float32, min, max int) (float32, float32) {
	if m := float32(max); v > m {
		return v - m, m
	}
	if m := float32(min); v < m {
		return v - m, m
	}
	return 0, v
}

// reset drops the frame's tree and ages the semantic ids.
//
// An id that went unclaimed this frame is dropped, and one that was claimed is
// marked free for the next frame to claim again. That is what keeps an id
// attached to a control for as long as the control keeps being drawn, and
// releases it when the control is gone.
func (q *pointerQueue) reset() {
	q.hitTree = q.hitTree[:0]
	q.areas = q.areas[:0]
	q.semantic.idsAssigned = false
	for k, ids := range q.semantic.contentIDs {
		for i := len(ids) - 1; i >= 0; i-- {
			if !ids[i].used {
				ids = slices.Delete(ids, i, i+1)
			} else {
				ids[i].used = false
			}
		}
		if len(ids) > 0 {
			q.semantic.contentIDs[k] = ids
		} else {
			delete(q.semantic.contentIDs, k)
		}
	}
}

// hitTest walks the frame's nodes from the top down, calling onNode for every
// node the point is inside. onNode returns false to stop the walk.
//
// The walk goes backwards through declaration order, which is drawing order,
// so the control drawn last -- the one the eye sees on top -- is asked first.
// When a node stops the walk, it continues from that node's enclosing node,
// which skips everything drawn underneath the control and still reaches the
// controls that contain it. That is the difference between two controls
// overlapping and one control inside another: the first is one receiver, the
// second is two, innermost first.
//
// It is shaped as a callback rather than a list of hits because routing,
// system actions and the accessibility tree want the same traversal and three
// different things out of it, and a list would build the union of the three
// on every question.
func (q *pointerQueue) hitTest(pos f32.Point, onNode func(*hitNode) bool) pointer.Cursor {
	// Track whether we're passing through hits.
	pass := true
	idx := len(q.hitTree) - 1
	cursor := pointer.CursorDefault
	for idx >= 0 {
		n := &q.hitTree[idx]
		hit, c := q.hit(n.area, pos)
		if !hit {
			idx--
			continue
		}
		// The first cursor found on the way down wins, which is the one
		// belonging to whatever is nearest the top.
		if cursor == pointer.CursorDefault {
			cursor = c
		}
		pass = pass && n.pass
		if pass {
			idx--
		} else {
			idx = n.next
		}
		if !onNode(n) {
			break
		}
	}
	return cursor
}

// hit reports whether pos is inside an area and every area enclosing it, and
// the cursor the innermost of them asked for.
//
// Every ancestor has to be satisfied, because an area is a clip: a control
// drawn half outside its scrolling parent is pressable only in the half that
// is visible, and a test that stopped at the control itself would make the
// hidden half answer too.
func (q *pointerQueue) hit(areaIdx int, p f32.Point) (bool, pointer.Cursor) {
	c := pointer.CursorDefault
	for areaIdx != -1 {
		a := &q.areas[areaIdx]
		if c == pointer.CursorDefault {
			c = a.cursor
		}
		// Each area is tested in the coordinates it was declared in, so the
		// point is carried back through that area's transform first.
		p := a.trans.Invert().Transform(p)
		if !a.area.Hit(p) {
			return false, c
		}
		areaIdx = a.parent
	}
	return true, c
}

// invTransform carries a window position into the coordinates of an area, so
// that a control reads a position against its own size and never has to know
// where on the window it was drawn.
func (q *pointerQueue) invTransform(areaIdx int, p f32.Point) f32.Point {
	if areaIdx == -1 {
		return p
	}
	return q.areas[areaIdx].trans.Invert().Transform(p)
}

// ActionAt returns the system action declared at pos, if any.
func (q *pointerQueue) ActionAt(pos f32.Point) (action system.Action, hasAction bool) {
	q.hitTest(pos, func(n *hitNode) bool {
		area := q.areas[n.area]
		if area.action != 0 {
			action = area.action
			hasAction = true
			return false
		}
		return true
	})
	return action, hasAction
}

// SemanticAt returns the id of the control at pos, if any.
func (q *pointerQueue) SemanticAt(pos f32.Point) (semID SemanticID, hasSemID bool) {
	q.assignSemIDs()
	q.hitTest(pos, func(n *hitNode) bool {
		area := q.areas[n.area]
		if area.semantic.id != 0 {
			semID = area.semantic.id
			hasSemID = true
			return false
		}
		return true
	})
	return semID, hasSemID
}

func (q *pointerQueue) assignSemIDs() {
	if q.semantic.idsAssigned {
		return
	}
	q.semantic.idsAssigned = true
	for i, a := range q.areas {
		if a.semantic.valid {
			q.areas[i].semantic.id = q.semanticIDFor(a.semantic.content)
		}
	}
}

// semanticIDFor hands back the id this content had on an earlier frame, or a
// fresh one.
//
// A list rather than a single id per content, because identical content is
// genuinely ambiguous -- two buttons reading "Delete" are described the same
// way -- and handing both the same id would merge two controls into one for
// anything reading the tree. The first unclaimed id in the list goes to the
// first of them declared, which keeps the pairing stable as long as the order
// is.
func (q *pointerQueue) semanticIDFor(content semanticContent) SemanticID {
	ids := q.semantic.contentIDs[content]
	for i, id := range ids {
		if !id.used {
			ids[i].used = true
			return id.id
		}
	}
	// No prior assigned ID; allocate a new one.
	q.semantic.lastID++
	id := semanticID{id: q.semantic.lastID, used: true}
	if q.semantic.contentIDs == nil {
		q.semantic.contentIDs = make(map[semanticContent][]semanticID)
	}
	q.semantic.contentIDs[content] = append(q.semantic.contentIDs[content], id)
	return id.id
}

// AppendSemantics appends the frame's accessibility tree to nodes.
func (q *pointerQueue) AppendSemantics(nodes []SemanticNode) []SemanticNode {
	q.assignSemIDs()
	nodes = q.appendSemanticChildren(nodes, 0)
	nodes = q.appendSemanticArea(nodes, 0, 0)
	return nodes
}

func (q *pointerQueue) appendSemanticArea(nodes []SemanticNode, parentID SemanticID, nodeIdx int) []SemanticNode {
	areaIdx := nodes[nodeIdx].areaIdx
	a := q.areas[areaIdx]
	childStart := len(nodes)
	nodes = q.appendSemanticChildren(nodes, a.firstChild)
	childEnd := len(nodes)
	for i := childStart; i < childEnd; i++ {
		nodes = q.appendSemanticArea(nodes, a.semantic.id, i)
	}
	n := &nodes[nodeIdx]
	n.ParentID = parentID
	n.Children = nodes[childStart:childEnd]
	return nodes
}

// appendSemanticChildren appends the described areas reachable from areaIdx,
// skipping past areas that describe nothing.
//
// An area with no description is not a level of the tree: clips exist for
// drawing, and a tree that reported every one of them would nest a button
// inside four anonymous boxes and report that as the shape of the interface.
func (q *pointerQueue) appendSemanticChildren(nodes []SemanticNode, areaIdx int) []SemanticNode {
	if areaIdx == -1 {
		return nodes
	}
	a := q.areas[areaIdx]
	if semID := a.semantic.id; semID != 0 {
		cnt := a.semantic.content
		nodes = append(nodes, SemanticNode{
			ID: semID,
			Desc: SemanticDesc{
				Bounds:      a.bounds(),
				Label:       cnt.label,
				Description: cnt.desc,
				Class:       cnt.class,
				Gestures:    cnt.gestures,
				Selected:    cnt.selected,
				Disabled:    cnt.disabled,
			},
			areaIdx: areaIdx,
		})
	} else {
		nodes = q.appendSemanticChildren(nodes, a.firstChild)
	}
	return q.appendSemanticChildren(nodes, a.sibling)
}

// SemanticArea returns the semantic content for area, and its parent area.
func (q *pointerQueue) SemanticArea(areaIdx int) (semanticContent, int) {
	for areaIdx != -1 {
		a := &q.areas[areaIdx]
		areaIdx = a.parent
		if !a.semantic.valid {
			continue
		}
		return a.semantic.content, areaIdx
	}
	return semanticContent{}, -1
}

// ClipFor clips r to the parents of area, which is how much of a control is
// actually on screen rather than how much of it was drawn.
func (q *pointerQueue) ClipFor(area int, r image.Rectangle) image.Rectangle {
	a := &q.areas[area]
	parent := a.parent
	for parent != -1 {
		a := &q.areas[parent]
		r = r.Intersect(a.bounds())
		parent = a.parent
	}
	return r
}

// Frame closes a frame: it derives what each handler can be pressed or
// scrolled by, and replays the last position of every live pointer against the
// tree that was just built.
//
// The replay is what makes a control light up under a pointer that never
// moved: the frame moved instead, and without it a menu that opened under the
// cursor would stay unhighlighted until the mouse was nudged.
func (q *pointerQueue) Frame(handlers map[event.Tag]*handler, state pointerState) (pointerState, []taggedEvent) {
	for _, h := range handlers {
		if h.pointer.areaPlusOne != 0 {
			area := &q.areas[h.pointer.areaPlusOne-1]
			// What a control announces it can do is derived from what it
			// asked to hear about, so that the two cannot disagree: a control
			// announced as pressable that filtered out presses would be a
			// promise to assistive technology that nothing keeps.
			if h.filter.pointer.kinds&(pointer.Press|pointer.Release) != 0 {
				area.semantic.content.gestures |= ClickGesture
			}
			if h.filter.pointer.kinds&pointer.Scroll != 0 {
				area.semantic.content.gestures |= ScrollGesture
			}
			area.semantic.valid = area.semantic.content.gestures != 0
		}
	}
	var evts []taggedEvent
	for i, p := range state.pointers {
		changed := false
		p, evts, state.cursor, changed = q.deliverEnterLeaveEvents(handlers, state.cursor, p, evts, p.last)
		if changed {
			state.pointers = slices.Clone(state.pointers)
			state.pointers[i] = p
		}
	}
	return state, evts
}

// Push routes one event from the platform.
func (q *pointerQueue) Push(handlers map[event.Tag]*handler, state pointerState, e pointer.Event) (pointerState, []taggedEvent) {
	var evts []taggedEvent
	if e.Kind == pointer.Cancel {
		// The system took the input away. Every handler is told, not only the
		// ones tracking a pointer, because a handler's belief that a gesture
		// is in progress is not something this queue can see.
		for k := range handlers {
			evts = append(evts, taggedEvent{
				event: pointer.Event{Kind: pointer.Cancel},
				tag:   k,
			})
		}
		state.pointers = nil
		return state, evts
	}
	if e.Kind == pointer.Scroll {
		// A scroll belongs to a position, not to a tracked pointer: a wheel
		// has nothing that goes down and comes up.
		return state, q.deliverScrollEvent(handlers, evts, e)
	}
	state, pidx := state.pointerOf(e)
	p := state.pointers[pidx]

	switch e.Kind {
	case pointer.Press:
		p, evts, state.cursor, _ = q.deliverEnterLeaveEvents(handlers, state.cursor, p, evts, e)
		p.pressed = true
		evts = q.deliverEvent(handlers, p, evts, e)
	case pointer.Move:
		// A move with the pointer down is a drag, told apart here so that a
		// control need not track pressedness to know which it is getting.
		if p.pressed {
			e.Kind = pointer.Drag
		}
		p, evts, state.cursor, _ = q.deliverEnterLeaveEvents(handlers, state.cursor, p, evts, e)
		evts = q.deliverEvent(handlers, p, evts, e)
		if p.pressed {
			p, evts = q.deliverDragEvent(handlers, p, evts)
		}
	case pointer.Leave:
		p, evts, state.cursor, _ = q.deliverEnterLeaveEvents(handlers, state.cursor, p, evts, e)
	case pointer.Release:
		// The release goes out before the set of handlers is recomputed, so
		// that it reaches whatever the press belonged to rather than whatever
		// the pointer happens to be over at the end.
		evts = q.deliverEvent(handlers, p, evts, e)
		p.pressed = false
		p, evts, state.cursor, _ = q.deliverEnterLeaveEvents(handlers, state.cursor, p, evts, e)
		p, evts = q.deliverDropEvent(handlers, p, evts)
	default:
		panic("unsupported pointer event type")
	}

	p.last = e

	if !p.pressed && len(p.entered) == 0 {
		// No longer need to track pointer.
		state.pointers = slices.Concat(state.pointers[:pidx:pidx], state.pointers[pidx+1:])
	} else {
		state.pointers = slices.Clone(state.pointers)
		state.pointers[pidx] = p
	}
	return state, evts
}

// pointerOf returns the pointerInfo index corresponding to the pointer in e,
// tracking it from now on if it is new.
func (s pointerState) pointerOf(e pointer.Event) (pointerState, int) {
	for i, p := range s.pointers {
		if p.id == e.PointerID {
			return s, i
		}
	}
	n := len(s.pointers)
	s.pointers = append(s.pointers[:n:n], pointerInfo{id: e.PointerID})
	return s, len(s.pointers) - 1
}

// grab gives one handler sole claim on a pressed pointer, and cancels every
// other handler that was receiving it.
//
// This is how an ambiguous gesture is settled: a press inside a list inside a
// button reaches both, and the one that decides the movement is a scroll takes
// the pointer so the button stops believing it is being pressed.
func (q *pointerQueue) grab(state pointerState, req pointer.GrabCmd) (pointerState, []taggedEvent) {
	var evts []taggedEvent
	for _, p := range state.pointers {
		if !p.pressed || p.id != req.ID {
			continue
		}
		// Verify that the grabber is among the handlers.
		found := slices.Contains(p.handlers, req.Tag)
		if !found {
			continue
		}
		// Drop other handlers that lost their grab.
		for _, tag := range slices.Backward(p.handlers) {
			if tag != req.Tag {
				evts = append(evts, taggedEvent{
					tag:   tag,
					event: pointer.Event{Kind: pointer.Cancel},
				})
				state = dropHandler(state, tag)
			}
		}
		break
	}
	return state, evts
}

// dropHandler removes tag from every pointer, so that nothing keeps reporting
// to a handler that has lost the pointer or is gone.
func dropHandler(state pointerState, tag event.Tag) pointerState {
	pointers := state.pointers
	state.pointers = nil
	for _, p := range pointers {
		handlers := p.handlers
		p.handlers = nil
		for _, h := range handlers {
			if h != tag {
				p.handlers = append(p.handlers, h)
			}
		}
		entered := p.entered
		p.entered = nil
		for _, h := range entered {
			if h != tag {
				p.entered = append(p.entered, h)
			}
		}
		state.pointers = append(state.pointers, p)
	}
	return state
}

// Deliver is like Push, but sends an event to one area and the areas
// containing it rather than to whatever is under a position. It is how an
// event aimed at a known control -- by the accessibility layer, say -- reaches
// it without a position being invented for it.
func (q *pointerQueue) Deliver(handlers map[event.Tag]*handler, areaIdx int, e pointer.Event) []taggedEvent {
	scroll := e.Scroll
	idx := len(q.hitTree) - 1
	// Locate first potential receiver.
	for idx != -1 {
		n := &q.hitTree[idx]
		if n.area == areaIdx {
			break
		}
		idx--
	}
	var evts []taggedEvent
	for idx != -1 {
		n := &q.hitTree[idx]
		idx = n.next
		h, ok := handlers[n.tag]
		if !ok || !h.filter.pointer.Matches(e) {
			continue
		}
		e := e
		if e.Kind == pointer.Scroll {
			if scroll == (f32.Point{}) {
				break
			}
			scroll, e.Scroll = h.filter.pointer.clampScroll(scroll)
		}
		e.Position = q.invTransform(h.pointer.areaPlusOne-1, e.Position)
		evts = append(evts, taggedEvent{tag: n.tag, event: e})
		// One receiver is enough for anything but a scroll, which carries on
		// outwards for as long as there is distance left over.
		if e.Kind != pointer.Scroll {
			break
		}
	}
	return evts
}

// deliverScrollEvent delivers a scroll to the handlers under its position.
func (q *pointerQueue) deliverScrollEvent(handlers map[event.Tag]*handler, evts []taggedEvent, e pointer.Event) []taggedEvent {
	var hits []event.Tag
	q.hitTest(e.Position, func(n *hitNode) bool {
		if _, ok := handlers[n.tag]; ok {
			hits = addHandler(hits, n.tag)
		}
		return true
	})
	return q.deliverEvent(handlers, pointerInfo{handlers: hits}, evts, e)
}

// deliverEvent sends e to the handlers the pointer reports to, innermost
// first.
func (q *pointerQueue) deliverEvent(handlers map[event.Tag]*handler, p pointerInfo, evts []taggedEvent, e pointer.Event) []taggedEvent {
	// A press with exactly one receiver is not ambiguous, and the receiver is
	// told so: there is nobody else who might take the gesture away.
	if p.pressed && len(p.handlers) == 1 {
		e.Priority = pointer.Grabbed
	}
	scroll := e.Scroll
	for _, k := range p.handlers {
		h, ok := handlers[k]
		if !ok {
			continue
		}
		f := h.filter.pointer
		if !f.Matches(e) {
			continue
		}
		if e.Kind == pointer.Scroll {
			// Nothing left to scroll by: the handlers further out get no
			// event rather than an event asking for no movement.
			if scroll == (f32.Point{}) {
				return evts
			}
			scroll, e.Scroll = f.clampScroll(scroll)
		}
		e := e
		e.Position = q.invTransform(h.pointer.areaPlusOne-1, e.Position)
		evts = append(evts, taggedEvent{event: e, tag: k})
	}
	return evts
}

// deliverEnterLeaveEvents recomputes who the pointer is inside and sends the
// enter and leave events that follow from the difference.
//
// While the pointer is pressed the set is not recomputed: the press belongs to
// what it went down on, and the only additions allowed are handlers that could
// accept the data being dragged, because a drop target has to light up under a
// finger that is already down.
func (q *pointerQueue) deliverEnterLeaveEvents(handlers map[event.Tag]*handler, cursor pointer.Cursor, p pointerInfo, evts []taggedEvent, e pointer.Event) (pointerInfo, []taggedEvent, pointer.Cursor, bool) {
	changed := false
	var hits []event.Tag
	if e.Kind == pointer.Leave || e.Source != pointer.Mouse && !p.pressed && e.Kind != pointer.Press {
		// A finger that is not touching the screen is nowhere. Only a mouse
		// hovers, so a released touch leaves everything it was inside rather
		// than staying where it last was.
	} else {
		var transSrc *pointerFilter
		if p.dataSource != nil {
			transSrc = &handlers[p.dataSource].filter.pointer
		}
		cursor = q.hitTest(e.Position, func(n *hitNode) bool {
			h, ok := handlers[n.tag]
			if !ok {
				return true
			}
			add := true
			if p.pressed {
				add = false
				// Filter out non-participating handlers,
				// except potential transfer targets when a transfer has been initiated.
				if _, found := searchTag(p.handlers, n.tag); found {
					add = true
				}
				if transSrc != nil {
					if _, ok := firstMimeMatch(transSrc, &h.filter.pointer); ok {
						add = true
					}
				}
			}
			if add {
				hits = addHandler(hits, n.tag)
			}
			return true
		})
		if !p.pressed {
			changed = true
			p.handlers = hits
		}
	}
	// Deliver Leave events.
	for _, k := range p.entered {
		if _, found := searchTag(hits, k); found {
			continue
		}
		h, ok := handlers[k]
		if !ok {
			continue
		}
		changed = true
		e := e
		e.Kind = pointer.Leave

		if h.filter.pointer.Matches(e) {
			e.Position = q.invTransform(h.pointer.areaPlusOne-1, e.Position)
			evts = append(evts, taggedEvent{tag: k, event: e})
		}
	}
	// Deliver Enter events.
	for _, k := range hits {
		if _, found := searchTag(p.entered, k); found {
			continue
		}
		h, ok := handlers[k]
		if !ok {
			continue
		}
		changed = true
		e := e
		e.Kind = pointer.Enter

		if h.filter.pointer.Matches(e) {
			e.Position = q.invTransform(h.pointer.areaPlusOne-1, e.Position)
			evts = append(evts, taggedEvent{tag: k, event: e})
		}
	}
	p.entered = hits
	return p, evts, cursor, changed
}

// deliverDragEvent starts a data transfer once a drag has begun over a handler
// that offers data, and tells every handler that could accept it.
func (q *pointerQueue) deliverDragEvent(handlers map[event.Tag]*handler, p pointerInfo, evts []taggedEvent) (pointerInfo, []taggedEvent) {
	if p.dataSource != nil {
		return p, evts
	}
	// Identify the data source.
	for _, k := range p.entered {
		src := &handlers[k].filter.pointer
		if len(src.sourceMimes) == 0 {
			continue
		}
		// One data source handler per pointer.
		p.dataSource = k
		// Notify all potential targets.
		for k, tgt := range handlers {
			if _, ok := firstMimeMatch(src, &tgt.filter.pointer); ok {
				evts = append(evts, taggedEvent{tag: k, event: transfer.InitiateEvent{}})
			}
		}
		break
	}
	return p, evts
}

// deliverDropEvent ends a transfer where the pointer came up: the first
// handler there that accepts a type the source offers becomes the target, and
// the source is asked for the data. A release over nothing that accepts it
// cancels the transfer rather than leaving it open.
func (q *pointerQueue) deliverDropEvent(handlers map[event.Tag]*handler, p pointerInfo, evts []taggedEvent) (pointerInfo, []taggedEvent) {
	if p.dataSource == nil {
		return p, evts
	}
	// Request data from the source.
	src := &handlers[p.dataSource].filter.pointer
	for _, k := range p.entered {
		h := handlers[k]
		if m, ok := firstMimeMatch(src, &h.filter.pointer); ok {
			p.dataTarget = k
			evts = append(evts, taggedEvent{tag: p.dataSource, event: transfer.RequestEvent{Type: m}})
			return p, evts
		}
	}
	// No valid target found, abort.
	return q.deliverTransferCancelEvent(handlers, p, evts)
}

// offerData hands the source's data to the target that asked for it, and ends
// the transfer either way.
func (q *pointerQueue) offerData(handlers map[event.Tag]*handler, state pointerState, req transfer.OfferCmd) (pointerState, []taggedEvent) {
	var evts []taggedEvent
	for i, p := range state.pointers {
		if p.dataSource != req.Tag {
			continue
		}
		if p.dataTarget != nil {
			evts = append(evts, taggedEvent{tag: p.dataTarget, event: transfer.DataEvent{
				Type: req.Type,
				Open: func() io.ReadCloser {
					return req.Data
				},
			}})
		}
		state.pointers = slices.Clone(state.pointers)
		state.pointers[i], evts = q.deliverTransferCancelEvent(handlers, p, evts)
		break
	}
	return state, evts
}

// deliverTransferCancelEvent closes a transfer down.
//
// Every handler that was told a transfer had begun is told it has ended, not
// only the source and the target, because the ones that lit up as possible
// targets have to stop looking like one.
func (q *pointerQueue) deliverTransferCancelEvent(handlers map[event.Tag]*handler, p pointerInfo, evts []taggedEvent) (pointerInfo, []taggedEvent) {
	evts = append(evts, taggedEvent{tag: p.dataSource, event: transfer.CancelEvent{}})
	// Cancel all potential targets.
	src := &handlers[p.dataSource].filter.pointer
	for k, h := range handlers {
		if _, ok := firstMimeMatch(src, &h.filter.pointer); ok {
			evts = append(evts, taggedEvent{tag: k, event: transfer.CancelEvent{}})
		}
	}
	p.dataSource = nil
	p.dataTarget = nil
	return p, evts
}

func searchTag(tags []event.Tag, tag event.Tag) (int, bool) {
	for i, t := range tags {
		if t == tag {
			return i, true
		}
	}
	return 0, false
}

// addHandler adds tag to the slice if not present. A handler declared twice in
// one frame is one handler, and would otherwise be told everything twice.
func addHandler(tags []event.Tag, tag event.Tag) []event.Tag {
	if slices.Contains(tags, tag) {
		return tags
	}
	return append(tags, tag)
}

// firstMimeMatch returns the first type match between src and tgt. The target's
// order decides, so a handler that would rather have one form of the data than
// another says so by the order it declares them in.
func firstMimeMatch(src, tgt *pointerFilter) (first string, matched bool) {
	for _, m1 := range tgt.targetMimes {
		if slices.Contains(src.sourceMimes, m1) {
			return m1, true
		}
	}
	return "", false
}

// Hit reports whether pos is inside the shape, in the shape's own coordinates.
//
// The rectangle is half open: the low edge belongs to it and the high edge
// does not. Two controls laid edge to edge share a line, and a rule that gave
// that line to both would make a press there reach whichever was declared last
// -- or both.
//
// The ellipse is the one inscribed in the same rectangle, and it is tested as
// an ellipse. This is the whole reason the shape is carried here at all: a
// round control whose hit test used its bounding rectangle would answer
// presses in the corners, where nothing is drawn, and no picture of the screen
// would show it.
func (op *areaOp) Hit(pos f32.Point) bool {
	pos = pos.Sub(f32internal.FPt(op.rect.Min))
	size := f32internal.FPt(op.rect.Size())
	switch op.kind {
	case areaRect:
		return 0 <= pos.X && pos.X < size.X &&
			0 <= pos.Y && pos.Y < size.Y
	case areaEllipse:
		rx := size.X / 2
		ry := size.Y / 2
		xh := pos.X - rx
		yk := pos.Y - ry
		// The ellipse function works in all cases because
		// 0/0 is not <= 1.
		return (xh*xh)/(rx*rx)+(yk*yk)/(ry*ry) <= 1
	default:
		panic("invalid area kind")
	}
}

// bounds is the area's extent in window coordinates, which is what anything
// outside this package -- an accessibility frame, a text input rectangle --
// needs, and is never what the hit test uses.
func (a *areaNode) bounds() image.Rectangle {
	return f32internal.Rectangle{
		Min: a.trans.Transform(f32internal.FPt(a.area.rect.Min)),
		Max: a.trans.Transform(f32internal.FPt(a.area.rect.Max)),
	}.Round()
}
