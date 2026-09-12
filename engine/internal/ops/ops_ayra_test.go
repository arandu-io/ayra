package ops

import (
	"encoding/binary"
	"image"
	"math"
	"reflect"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/internal/scene"
)

type wireContract struct {
	typ  OpType
	size uint32
	refs uint32
	name string
}

var wireContracts = []wireContract{
	{TypeMacro, TypeMacroLen, 0, "Macro"},
	{TypeCall, TypeCallLen, 1, "Call"},
	{TypeDefer, TypeDeferLen, 0, "Defer"},
	{TypeTransform, TypeTransformLen, 0, "Transform"},
	{TypePopTransform, TypePopTransformLen, 0, "PopTransform"},
	{TypePushOpacity, TypePushOpacityLen, 0, "PushOpacity"},
	{TypePopOpacity, TypePopOpacityLen, 0, "PopOpacity"},
	{TypeImage, TypeImageLen, 2, "Image"},
	{TypePaint, TypePaintLen, 0, "Paint"},
	{TypeColor, TypeColorLen, 0, "Color"},
	{TypeLinearGradient, TypeLinearGradientLen, 0, "LinearGradient"},
	{TypePass, TypePassLen, 0, "Pass"},
	{TypePopPass, TypePopPassLen, 0, "PopPass"},
	{TypeInput, TypeInputLen, 1, "Input"},
	{TypeKeyInputHint, TypeKeyInputHintLen, 1, "KeyInputHint"},
	{TypeSave, TypeSaveLen, 0, "Save"},
	{TypeLoad, TypeLoadLen, 0, "Load"},
	{TypeAux, TypeAuxLen, 0, "Aux"},
	{TypeClip, TypeClipLen, 0, "Clip"},
	{TypePopClip, TypePopClipLen, 0, "PopClip"},
	{TypeCursor, TypeCursorLen, 0, "Cursor"},
	{TypePath, TypePathLen, 0, "Path"},
	{TypeStroke, TypeStrokeLen, 0, "Stroke"},
	{TypeSemanticLabel, TypeSemanticLabelLen, 1, "SemanticLabel"},
	{TypeSemanticDesc, TypeSemanticDescLen, 1, "SemanticDesc"},
	{TypeSemanticClass, TypeSemanticClassLen, 0, "SemanticClass"},
	{TypeSemanticSelected, TypeSemanticSelectedLen, 0, "SemanticSelected"},
	{TypeSemanticEnabled, TypeSemanticEnabledLen, 0, "SemanticEnabled"},
	{TypeActionInput, TypeActionInputLen, 0, "ActionInput"},
}

// These numbers are the wire format. Moving a declaration in the constant
// block must not silently teach a new reader a different language.
func TestEveryOperationKeepsItsWireNumberSizeAndReferences(t *testing.T) {
	for at, contract := range wireContracts {
		wantNumber := OpType(firstOpIndex + at)
		if contract.typ != wantNumber {
			t.Errorf("%s is byte %d, want %d", contract.name, contract.typ, wantNumber)
		}
		if got := contract.typ.Size(); got != contract.size {
			t.Errorf("%s occupies %d bytes, want %d", contract.name, got, contract.size)
		}
		if got := contract.typ.NumRefs(); got != contract.refs {
			t.Errorf("%s consumes %d references, want %d", contract.name, got, contract.refs)
		}
		if got := contract.typ.String(); got != contract.name {
			t.Errorf("byte %d is named %q, want %q", contract.typ, got, contract.name)
		}
	}
}

func TestAnUnknownOperationHasNoShapeAndStillHasAName(t *testing.T) {
	unknown := OpType(17)
	if unknown.Size() != 0 || unknown.NumRefs() != 0 {
		t.Fatalf("unknown operation reports size %d and %d references", unknown.Size(), unknown.NumRefs())
	}
	if got, want := unknown.String(), "OpType(17)"; got != want {
		t.Errorf("unknown operation is named %q, want %q", got, want)
	}
}

func ordinaryContract(contract wireContract) bool {
	switch contract.typ {
	case TypeMacro, TypeCall, TypeDefer, TypeAux:
		return false
	default:
		return true
	}
}

func appendWireOp(o *Ops, contract wireContract, marker byte) ([]byte, []any) {
	refs := []any{"first", image.Pt(7, 9)}
	var data []byte
	switch contract.refs {
	case 0:
		data = Write(o, int(contract.size))
	case 1:
		data = Write1(o, int(contract.size), refs[0])
	case 2:
		data = Write2(o, int(contract.size), refs[0], refs[1])
	default:
		panic("test contract has too many references")
	}
	data[0] = byte(contract.typ)
	for i := 1; i < len(data); i++ {
		data[i] = marker + byte(i)
	}
	return append([]byte(nil), data...), append([]any(nil), refs[:contract.refs]...)
}

// The reader is where byte widths and reference counts meet. Each ordinary
// operation carries distinct bytes so a one-byte stride error cannot hide.
func TestOrdinaryOperationsRoundTripInOrder(t *testing.T) {
	var stream Ops
	type expected struct {
		data []byte
		refs []any
	}
	var want []expected
	for at, contract := range wireContracts {
		if !ordinaryContract(contract) {
			continue
		}
		data, refs := appendWireOp(&stream, contract, byte(at*3))
		want = append(want, expected{data: data, refs: refs})
	}

	var reader Reader
	reader.Reset(&stream)
	for at, expected := range want {
		got, ok := reader.Decode()
		if !ok {
			t.Fatalf("stream ended before operation %d", at)
		}
		if !reflect.DeepEqual(got.Data, expected.data) {
			t.Errorf("operation %d data is %v, want %v", at, got.Data, expected.data)
		}
		if len(got.Refs) != len(expected.refs) || (len(got.Refs) > 0 && !reflect.DeepEqual(got.Refs, expected.refs)) {
			t.Errorf("operation %d references are %v, want %v", at, got.Refs, expected.refs)
		}
	}
	if extra, ok := reader.Decode(); ok {
		t.Errorf("reader invented an operation after the stream: %v", extra.Data)
	}
}

func TestEmptyUnknownAndTruncatedStreamsEndCleanly(t *testing.T) {
	assertEnds := func(t *testing.T, o *Ops) {
		t.Helper()
		defer func() {
			if state := recover(); state != nil {
				t.Errorf("reader panicked instead of ending: %v", state)
			}
		}()
		var reader Reader
		reader.Reset(o)
		if got, ok := reader.Decode(); ok {
			t.Errorf("partial stream yielded %v", got.Data)
		}
	}

	assertEnds(t, &Ops{})
	assertEnds(t, &Ops{data: []byte{17}})
	for _, contract := range wireContracts {
		if !ordinaryContract(contract) {
			continue
		}
		for length := 1; length < int(contract.size); length++ {
			t.Run(contract.name+"/bytes", func(t *testing.T) {
				data := make([]byte, length)
				data[0] = byte(contract.typ)
				assertEnds(t, &Ops{data: data})
			})
		}
		if contract.refs > 0 {
			t.Run(contract.name+"/references", func(t *testing.T) {
				data := make([]byte, contract.size)
				data[0] = byte(contract.typ)
				refs := make([]any, contract.refs-1)
				assertEnds(t, &Ops{data: data, refs: refs})
			})
		}
	}
}

func TestMalformedStructuralOperationsEndCleanly(t *testing.T) {
	assertEnds := func(t *testing.T, o *Ops) {
		t.Helper()
		defer func() {
			if state := recover(); state != nil {
				t.Errorf("malformed structural operation panicked: %v", state)
			}
		}()
		var reader Reader
		reader.Reset(o)
		if _, ok := reader.Decode(); ok {
			t.Error("malformed structural operation was decoded")
		}
	}

	for length := 1; length < TypeMacroLen; length++ {
		data := make([]byte, length)
		data[0] = byte(TypeMacro)
		assertEnds(t, &Ops{data: data})
	}
	for length := 1; length < TypeCallLen; length++ {
		data := make([]byte, length)
		data[0] = byte(TypeCall)
		assertEnds(t, &Ops{data: data})
	}
	badCall := make([]byte, TypeCallLen)
	badCall[0] = byte(TypeCall)
	assertEnds(t, &Ops{data: badCall})
	assertEnds(t, &Ops{data: badCall, refs: []any{"not an operation stream"}})
	assertEnds(t, &Ops{data: []byte{byte(TypeAux)}})

	badMacro := make([]byte, TypeMacroLen)
	badMacro[0] = byte(TypeMacro)
	binary.LittleEndian.PutUint32(badMacro[1:], 99)
	assertEnds(t, &Ops{data: badMacro})
}

func TestAnIncompleteRecordingKeepsItsContentsFromExecuting(t *testing.T) {
	var o Ops
	_, _ = beginRecording(&o)
	Write(&o, TypePaintLen)[0] = byte(TypePaint)

	var reader Reader
	reader.Reset(&o)
	if op, ok := reader.Decode(); ok {
		t.Errorf("unfinished recording leaked operation %v", op.Data)
	}
}

func TestResetAtBeginsAtTheRequestedInstructionAndReference(t *testing.T) {
	var o Ops
	first := Write1(&o, TypeInputLen, "first")
	first[0] = byte(TypeInput)
	start := PCFor(&o)
	second := Write1(&o, TypeInputLen, "second")
	second[0] = byte(TypeInput)

	var reader Reader
	reader.ResetAt(&o, start)
	got, ok := reader.Decode()
	if !ok || len(got.Refs) != 1 || got.Refs[0] != "second" {
		t.Fatalf("reader starting at %v answered data=%v refs=%v ok=%v", start, got.Data, got.Refs, ok)
	}
	if _, ok := reader.Decode(); ok {
		t.Error("reader continued past the stream")
	}
}

func TestReferenceWritersKeepReferenceOrderAndStringValue(t *testing.T) {
	var o Ops
	Write1String(&o, 1, "one")
	Write2(&o, 1, 2, 3)
	Write2String(&o, 1, 4, "five")
	Write3(&o, 1, 6, 7, 8)

	if len(o.refs) != 8 {
		t.Fatalf("writers stored %d references, want 8", len(o.refs))
	}
	got := []any{*(o.refs[0].(*string)), o.refs[1], o.refs[2], o.refs[3], *(o.refs[4].(*string)), o.refs[5], o.refs[6], o.refs[7]}
	want := []any{"one", 2, 3, 4, "five", 6, 7, 8}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("references are %v, want %v", got, want)
	}
}

func beginRecording(o *Ops) (StackID, PC) {
	id := PushMacro(o)
	start := PCFor(o)
	data := Write(o, TypeMacroLen)
	data[0] = byte(TypeMacro)
	return id, start
}

func endRecording(o *Ops, id StackID, header PC) (PC, PC) {
	PopMacro(o, id)
	FillMacro(o, header)
	return header.Add(TypeMacro), PCFor(o)
}

func TestNestedRecordingsReplayTheirContentsOnce(t *testing.T) {
	var source Ops
	outerID, outerHeader := beginRecording(&source)
	outerData := Write(&source, TypeColorLen)
	outerData[0] = byte(TypeColor)
	innerID, innerHeader := beginRecording(&source)
	innerData := Write(&source, TypePaintLen)
	innerData[0] = byte(TypePaint)
	innerStart, innerEnd := endRecording(&source, innerID, innerHeader)
	AddCall(&source, &source, innerStart, innerEnd)
	outerStart, outerEnd := endRecording(&source, outerID, outerHeader)

	var destination Ops
	AddCall(&destination, &source, outerStart, outerEnd)
	var reader Reader
	reader.Reset(&destination)
	var got []OpType
	for {
		op, ok := reader.Decode()
		if !ok {
			break
		}
		got = append(got, OpType(op.Data[0]))
	}
	if want := []OpType{TypeColor, TypePaint}; !reflect.DeepEqual(got, want) {
		t.Errorf("nested recording replayed %v, want %v", got, want)
	}
}

func TestDeferredCallsRunAfterTheOrdinaryStreamInTheirOriginalOrder(t *testing.T) {
	var source Ops
	makeCall := func(typ OpType) (PC, PC) {
		id, header := beginRecording(&source)
		data := Write(&source, int(typ.Size()))
		data[0] = byte(typ)
		return endRecording(&source, id, header)
	}
	firstStart, firstEnd := makeCall(TypeColor)
	secondStart, secondEnd := makeCall(TypePaint)

	var destination Ops
	for _, call := range []struct{ start, end PC }{{firstStart, firstEnd}, {secondStart, secondEnd}} {
		marker := Write(&destination, TypeDeferLen)
		marker[0] = byte(TypeDefer)
		AddCall(&destination, &source, call.start, call.end)
	}
	ordinary := Write(&destination, TypePopClipLen)
	ordinary[0] = byte(TypePopClip)

	var reader Reader
	reader.Reset(&destination)
	var got []OpType
	for {
		op, ok := reader.Decode()
		if !ok {
			break
		}
		got = append(got, OpType(op.Data[0]))
	}
	if want := []OpType{TypePopClip, TypeColor, TypePaint}; !reflect.DeepEqual(got, want) {
		t.Errorf("deferred stream ran as %v, want %v", got, want)
	}
}

func TestResetReturnsEveryWriterStateToZero(t *testing.T) {
	var o Ops
	Write1String(&o, TypeSemanticLabelLen, "old frame")[0] = byte(TypeSemanticLabel)
	PushOp(&o, ClipStack)
	PushMacro(&o)
	BeginMulti(&o)
	oldVersion := o.version

	Reset(&o)

	if len(o.data) != 0 || len(o.refs) != 0 || len(o.stringRefs) != 0 {
		t.Errorf("reset retained %d bytes, %d references and %d strings", len(o.data), len(o.refs), len(o.stringRefs))
	}
	if o.version != oldVersion+1 || o.nextStateID != 0 {
		t.Errorf("reset left version %d and next state %d", o.version, o.nextStateID)
	}
	defer func() {
		if state := recover(); state != nil {
			t.Fatalf("first ordinary write after reset panicked: %v", state)
		}
	}()
	Write(&o, TypePaintLen)[0] = byte(TypePaint)
	id, macroID := PushOp(&o, ClipStack)
	PopOp(&o, ClipStack, id, macroID)
	macro := PushMacro(&o)
	PopMacro(&o, macro)
}

func TestStackEntriesMustCloseInOrderAndInsideTheirRecording(t *testing.T) {
	assertPanics := func(t *testing.T, action func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Error("unbalanced stack operation was accepted")
			}
		}()
		action()
	}
	var o Ops
	first, firstMacro := PushOp(&o, TransStack)
	second, secondMacro := PushOp(&o, TransStack)
	assertPanics(t, func() { PopOp(&o, TransStack, first, firstMacro) })
	PopOp(&o, TransStack, second, secondMacro)
	PopOp(&o, TransStack, first, firstMacro)

	stack, macroID := PushOp(&o, ClipStack)
	recording := PushMacro(&o)
	assertPanics(t, func() { PopOp(&o, ClipStack, stack, macroID) })
	PopMacro(&o, recording)
	PopOp(&o, ClipStack, stack, macroID)
}

func TestMultiWritesCannotMixWithOrdinaryWrites(t *testing.T) {
	assertPanics := func(t *testing.T, action func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Error("invalid multi-operation transition was accepted")
			}
		}()
		action()
	}
	var o Ops
	assertPanics(t, func() { WriteMulti(&o, 1) })
	assertPanics(t, func() { EndMulti(&o) })
	BeginMulti(&o)
	assertPanics(t, func() { BeginMulti(&o) })
	assertPanics(t, func() { Write(&o, 1) })
	WriteMulti(&o, 1)
	EndMulti(&o)
	Write(&o, 1)
}

func TestSaveAndLoadCarryTheSameMonotonicIdentity(t *testing.T) {
	var o Ops
	first := Save(&o)
	first.Load()
	second := Save(&o)
	second.Load()

	var reader Reader
	reader.Reset(&o)
	var got []int
	for {
		op, ok := reader.Decode()
		if !ok {
			break
		}
		switch OpType(op.Data[0]) {
		case TypeSave:
			got = append(got, DecodeSave(op.Data))
		case TypeLoad:
			got = append(got, DecodeLoad(op.Data))
		}
	}
	if want := []int{1, 1, 2, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("save/load identities are %v, want %v", got, want)
	}
}

func TestTypedDecodersRecoverTheValuesOnTheWire(t *testing.T) {
	clipData := make([]byte, TypeClipLen)
	clipData[0] = byte(TypeClip)
	bo := binary.LittleEndian
	for at, value := range []int32{-11, 22, 33, -44} {
		bo.PutUint32(clipData[1+at*4:], uint32(value))
	}
	clipData[17] = 1
	clipData[18] = byte(Ellipse)
	var clipped ClipOp
	clipped.Decode(clipData)
	wantClip := ClipOp{
		Bounds:  image.Rectangle{Min: image.Pt(-11, 22), Max: image.Pt(33, -44)},
		Outline: true,
		Shape:   Ellipse,
	}
	if clipped != wantClip {
		t.Errorf("clip decoded as %+v, want %+v", clipped, wantClip)
	}

	transformData := make([]byte, TypeTransformLen)
	transformData[0], transformData[1] = byte(TypeTransform), 1
	for at, value := range []float32{1, 2, 3, 4, 5, 6} {
		bo.PutUint32(transformData[2+at*4:], math.Float32bits(value))
	}
	transform, push := DecodeTransform(transformData)
	if moved := transform.Transform(f32.Pt(7, 8)); !push || moved != f32.Pt(1*7+2*8+3, 4*7+5*8+6) {
		t.Errorf("transform moved to %v with push=%v", moved, push)
	}

	opacityData := make([]byte, TypePushOpacityLen)
	opacityData[0] = byte(TypePushOpacity)
	bo.PutUint32(opacityData[1:], math.Float32bits(.375))
	if got := DecodeOpacity(opacityData); got != .375 {
		t.Errorf("opacity decoded as %v", got)
	}
}

func TestSceneCommandsSurviveTheByteStream(t *testing.T) {
	var command scene.Command
	for i := range command {
		command[i] = uint32(i*101 + 7)
	}
	encoded := make([]byte, scene.CommandSize)
	EncodeCommand(encoded, command)
	if got := DecodeCommand(encoded); got != command {
		t.Errorf("scene command decoded as %v, want %v", got, command)
	}
}
