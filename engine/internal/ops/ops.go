package ops

import (
	"encoding/binary"
	"fmt"
	"image"
	"math"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/internal/byteslice"
	"github.com/arandu-io/ayra/engine/internal/scene"
)

type Ops struct {
	// version is incremented at each Reset.
	version uint32
	// data contains the serialized operations.
	data []byte
	// refs hold external references for operations.
	refs []any
	// stringRefs provides space for string references, pointers to which will
	// be stored in refs. Storing a string directly in refs would cause a heap
	// allocation, to store the string header in an interface value. The backing
	// array of stringRefs, on the other hand, gets reused between calls to
	// reset, making string references free on average.
	//
	// Appending to stringRefs might reallocate the backing array, which will
	// leave pointers to the old array in refs. This temporarily causes a slight
	// increase in memory usage, but this, too, amortizes away as the capacity
	// of stringRefs approaches its stable maximum.
	stringRefs []string
	// nextStateID is the id allocated for the next
	// StateOp.
	nextStateID uint32
	// multipOp indicates a multi-op such as clip.Path is being added.
	multipOp bool

	macroStack stack
	stacks     [_StackKind]stack
}

type OpType byte

type Shape byte

// Start at a high number for easier debugging.
const firstOpIndex = 200

const (
	TypeMacro OpType = iota + firstOpIndex
	TypeCall
	TypeDefer
	TypeTransform
	TypePopTransform
	TypePushOpacity
	TypePopOpacity
	TypeImage
	TypePaint
	TypeColor
	TypeLinearGradient
	TypePass
	TypePopPass
	TypeInput
	TypeKeyInputHint
	TypeSave
	TypeLoad
	TypeAux
	TypeClip
	TypePopClip
	TypeCursor
	TypePath
	TypeStroke
	TypeSemanticLabel
	TypeSemanticDesc
	TypeSemanticClass
	TypeSemanticSelected
	TypeSemanticEnabled
	TypeActionInput
)

type StackID struct {
	id   uint32
	prev uint32
}

// StateOp represents a saved operation snapshot to be restored
// later.
type StateOp struct {
	id      uint32
	macroID uint32
	ops     *Ops
}

// stack tracks the integer identities of stack operations to ensure correct
// pairing of their push and pop methods.
type stack struct {
	currentID uint32
	nextID    uint32
}

type StackKind uint8

// ClipOp is the shadow of clip.Op.
type ClipOp struct {
	Bounds  image.Rectangle
	Outline bool
	Shape   Shape
}

const (
	ClipStack StackKind = iota
	TransStack
	PassStack
	OpacityStack
	_StackKind
)

const (
	Path Shape = iota
	Ellipse
	Rect
)

const (
	TypeMacroLen            = 1 + 4 + 4
	TypeCallLen             = 1 + 4 + 4 + 4 + 4
	TypeDeferLen            = 1
	TypeTransformLen        = 1 + 1 + 4*6
	TypePopTransformLen     = 1
	TypePushOpacityLen      = 1 + 4
	TypePopOpacityLen       = 1
	TypeRedrawLen           = 1 + 8
	TypeImageLen            = 1 + 1
	TypePaintLen            = 1
	TypeColorLen            = 1 + 4
	TypeLinearGradientLen   = 1 + 8*2 + 4*2
	TypePassLen             = 1
	TypePopPassLen          = 1
	TypeInputLen            = 1
	TypeKeyInputHintLen     = 1 + 1
	TypeSaveLen             = 1 + 4
	TypeLoadLen             = 1 + 4
	TypeAuxLen              = 1
	TypeClipLen             = 1 + 4*4 + 1 + 1
	TypePopClipLen          = 1
	TypeCursorLen           = 2
	TypePathLen             = 8 + 1
	TypeStrokeLen           = 1 + 4
	TypeSemanticLabelLen    = 1
	TypeSemanticDescLen     = 1
	TypeSemanticClassLen    = 2
	TypeSemanticSelectedLen = 2
	TypeSemanticEnabledLen  = 2
	TypeActionInputLen      = 1 + 1
)

func (op *ClipOp) Decode(data []byte) {
	requireOperation(data, TypeClip, TypeClipLen)
	data = data[:TypeClipLen]
	bo := binary.LittleEndian
	op.Bounds.Min.X = int(int32(bo.Uint32(data[1:])))
	op.Bounds.Min.Y = int(int32(bo.Uint32(data[5:])))
	op.Bounds.Max.X = int(int32(bo.Uint32(data[9:])))
	op.Bounds.Max.Y = int(int32(bo.Uint32(data[13:])))
	op.Outline = data[17] == 1
	op.Shape = Shape(data[18])
}

func Reset(o *Ops) {
	o.macroStack = stack{}
	o.stacks = [_StackKind]stack{}
	// Leave references to the GC.
	for i := range o.refs {
		o.refs[i] = nil
	}
	for i := range o.stringRefs {
		o.stringRefs[i] = ""
	}
	o.data = o.data[:0]
	o.refs = o.refs[:0]
	o.stringRefs = o.stringRefs[:0]
	o.nextStateID = 0
	o.multipOp = false
	o.version++
}

func reserve(o *Ops, n int) []byte {
	if n < 0 {
		panic("negative operation size")
	}
	start := len(o.data)
	o.data = append(o.data, make([]byte, n)...)
	return o.data[start:]
}

func Write(o *Ops, n int) []byte {
	if o.multipOp {
		panic("cannot mix multi ops with single ones")
	}
	return reserve(o, n)
}

func BeginMulti(o *Ops) {
	if o.multipOp {
		panic("cannot interleave multi ops")
	}
	o.multipOp = true
}

func EndMulti(o *Ops) {
	if !o.multipOp {
		panic("cannot end non multi ops")
	}
	o.multipOp = false
}

func WriteMulti(o *Ops, n int) []byte {
	if !o.multipOp {
		panic("cannot use multi ops in single ops")
	}
	return reserve(o, n)
}

func PushMacro(o *Ops) StackID {
	return o.macroStack.push()
}

func PopMacro(o *Ops, id StackID) {
	o.macroStack.pop(id)
}

func FillMacro(o *Ops, startPC PC) {
	pc := PCFor(o)
	// Fill out the macro definition reserved in Record.
	data := o.data[startPC.data:]
	data = data[:TypeMacroLen]
	data[0] = byte(TypeMacro)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(pc.data))
	bo.PutUint32(data[5:], uint32(pc.refs))
}

func AddCall(o *Ops, callOps *Ops, pc PC, end PC) {
	data := Write1(o, TypeCallLen, callOps)
	data[0] = byte(TypeCall)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(pc.data))
	bo.PutUint32(data[5:], uint32(pc.refs))
	bo.PutUint32(data[9:], uint32(end.data))
	bo.PutUint32(data[13:], uint32(end.refs))
}

func PushOp(o *Ops, kind StackKind) (StackID, uint32) {
	return o.stacks[kind].push(), o.macroStack.currentID
}

func PopOp(o *Ops, kind StackKind, sid StackID, macroID uint32) {
	if o.macroStack.currentID != macroID {
		panic("stack push and pop must not cross macro boundary")
	}
	o.stacks[kind].pop(sid)
}

func Write1(o *Ops, n int, ref1 any) []byte {
	data := Write(o, n)
	o.refs = append(o.refs, ref1)
	return data
}

func Write1String(o *Ops, n int, ref1 string) []byte {
	data := Write(o, n)
	o.stringRefs = append(o.stringRefs, ref1)
	o.refs = append(o.refs, &o.stringRefs[len(o.stringRefs)-1])
	return data
}

func Write2(o *Ops, n int, ref1, ref2 any) []byte {
	data := Write(o, n)
	o.refs = append(o.refs, ref1, ref2)
	return data
}

func Write2String(o *Ops, n int, ref1 any, ref2 string) []byte {
	data := Write(o, n)
	o.stringRefs = append(o.stringRefs, ref2)
	o.refs = append(o.refs, ref1, &o.stringRefs[len(o.stringRefs)-1])
	return data
}

func Write3(o *Ops, n int, ref1, ref2, ref3 any) []byte {
	data := Write(o, n)
	o.refs = append(o.refs, ref1, ref2, ref3)
	return data
}

func PCFor(o *Ops) PC {
	return PC{data: uint32(len(o.data)), refs: uint32(len(o.refs))}
}

func (s *stack) push() StackID {
	s.nextID++
	sid := StackID{
		id:   s.nextID,
		prev: s.currentID,
	}
	s.currentID = s.nextID
	return sid
}

func (s *stack) check(sid StackID) {
	if s.currentID != sid.id {
		panic("unbalanced operation")
	}
}

func (s *stack) pop(sid StackID) {
	s.check(sid)
	s.currentID = sid.prev
}

// Save the effective transformation.
func Save(o *Ops) StateOp {
	o.nextStateID++
	s := StateOp{
		ops:     o,
		id:      o.nextStateID,
		macroID: o.macroStack.currentID,
	}
	bo := binary.LittleEndian
	data := Write(o, TypeSaveLen)
	data[0] = byte(TypeSave)
	bo.PutUint32(data[1:], uint32(s.id))
	return s
}

// Load a previously saved operations state given
// its ID.
func (s StateOp) Load() {
	bo := binary.LittleEndian
	data := Write(s.ops, TypeLoadLen)
	data[0] = byte(TypeLoad)
	bo.PutUint32(data[1:], uint32(s.id))
}

func DecodeCommand(d []byte) scene.Command {
	var cmd scene.Command
	copy(byteslice.Slice(cmd[:]), d)
	return cmd
}

func EncodeCommand(out []byte, cmd scene.Command) {
	copy(out, byteslice.Slice(cmd[:]))
}

func DecodeTransform(data []byte) (t f32.Affine2D, push bool) {
	requireOperation(data, TypeTransform, TypeTransformLen)
	push = data[1] != 0
	data = data[2:]
	data = data[:4*6]

	bo := binary.LittleEndian
	a := math.Float32frombits(bo.Uint32(data))
	b := math.Float32frombits(bo.Uint32(data[4*1:]))
	c := math.Float32frombits(bo.Uint32(data[4*2:]))
	d := math.Float32frombits(bo.Uint32(data[4*3:]))
	e := math.Float32frombits(bo.Uint32(data[4*4:]))
	f := math.Float32frombits(bo.Uint32(data[4*5:]))
	return f32.NewAffine2D(a, b, c, d, e, f), push
}

func DecodeOpacity(data []byte) float32 {
	requireOperation(data, TypePushOpacity, TypePushOpacityLen)
	bo := binary.LittleEndian
	return math.Float32frombits(bo.Uint32(data[1:]))
}

// DecodeSave decodes the state id of a save op.
func DecodeSave(data []byte) int {
	requireOperation(data, TypeSave, TypeSaveLen)
	bo := binary.LittleEndian
	return int(bo.Uint32(data[1:]))
}

// DecodeLoad decodes the state id of a load op.
func DecodeLoad(data []byte) int {
	requireOperation(data, TypeLoad, TypeLoadLen)
	bo := binary.LittleEndian
	return int(bo.Uint32(data[1:]))
}

type opProp struct {
	Size    byte
	NumRefs byte
	Name    string
}

var opProps = [0x100]opProp{
	TypeMacro:            {Size: TypeMacroLen, Name: "Macro"},
	TypeCall:             {Size: TypeCallLen, NumRefs: 1, Name: "Call"},
	TypeDefer:            {Size: TypeDeferLen, Name: "Defer"},
	TypeTransform:        {Size: TypeTransformLen, Name: "Transform"},
	TypePopTransform:     {Size: TypePopTransformLen, Name: "PopTransform"},
	TypePushOpacity:      {Size: TypePushOpacityLen, Name: "PushOpacity"},
	TypePopOpacity:       {Size: TypePopOpacityLen, Name: "PopOpacity"},
	TypeImage:            {Size: TypeImageLen, NumRefs: 2, Name: "Image"},
	TypePaint:            {Size: TypePaintLen, Name: "Paint"},
	TypeColor:            {Size: TypeColorLen, Name: "Color"},
	TypeLinearGradient:   {Size: TypeLinearGradientLen, Name: "LinearGradient"},
	TypePass:             {Size: TypePassLen, Name: "Pass"},
	TypePopPass:          {Size: TypePopPassLen, Name: "PopPass"},
	TypeInput:            {Size: TypeInputLen, NumRefs: 1, Name: "Input"},
	TypeKeyInputHint:     {Size: TypeKeyInputHintLen, NumRefs: 1, Name: "KeyInputHint"},
	TypeSave:             {Size: TypeSaveLen, Name: "Save"},
	TypeLoad:             {Size: TypeLoadLen, Name: "Load"},
	TypeAux:              {Size: TypeAuxLen, Name: "Aux"},
	TypeClip:             {Size: TypeClipLen, Name: "Clip"},
	TypePopClip:          {Size: TypePopClipLen, Name: "PopClip"},
	TypeCursor:           {Size: TypeCursorLen, Name: "Cursor"},
	TypePath:             {Size: TypePathLen, Name: "Path"},
	TypeStroke:           {Size: TypeStrokeLen, Name: "Stroke"},
	TypeSemanticLabel:    {Size: TypeSemanticLabelLen, NumRefs: 1, Name: "SemanticLabel"},
	TypeSemanticDesc:     {Size: TypeSemanticDescLen, NumRefs: 1, Name: "SemanticDesc"},
	TypeSemanticClass:    {Size: TypeSemanticClassLen, Name: "SemanticClass"},
	TypeSemanticSelected: {Size: TypeSemanticSelectedLen, Name: "SemanticSelected"},
	TypeSemanticEnabled:  {Size: TypeSemanticEnabledLen, Name: "SemanticEnabled"},
	TypeActionInput:      {Size: TypeActionInputLen, Name: "ActionInput"},
}

func requireOperation(data []byte, typ OpType, size int) {
	if len(data) < size || OpType(data[0]) != typ {
		panic("invalid op")
	}
}

func (t OpType) props() (size, numRefs uint32) {
	v := opProps[t]
	return uint32(v.Size), uint32(v.NumRefs)
}

func (t OpType) Size() uint32 {
	return uint32(opProps[t].Size)
}

func (t OpType) NumRefs() uint32 {
	return uint32(opProps[t].NumRefs)
}

func (t OpType) String() string {
	if name := opProps[t].Name; name != "" {
		return name
	}
	return fmt.Sprintf("OpType(%d)", byte(t))
}
