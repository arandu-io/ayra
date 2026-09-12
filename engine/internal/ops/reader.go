package ops

import (
	"encoding/binary"
)

// Reader parses an ops list.
type Reader struct {
	pc        PC
	stack     []macro
	ops       *Ops
	deferOps  Ops
	deferDone bool
}

// EncodedOp represents an encoded op returned by
// Reader.
type EncodedOp struct {
	Key  Key
	Data []byte
	Refs []any
}

// Key is a unique key for a given op.
type Key struct {
	ops     *Ops
	pc      uint32
	version uint32
}

// Shadow of op.MacroOp.
type macroOp struct {
	ops   *Ops
	start PC
	end   PC
}

// PC is an instruction counter for an operation list.
type PC struct {
	data uint32
	refs uint32
}

type macro struct {
	ops   *Ops
	retPC PC
	endPC PC
}

type opMacroDef struct {
	endpc PC
}

func validPC(o *Ops, pc PC) bool {
	return o != nil && pc.data <= uint32(len(o.data)) && pc.refs <= uint32(len(o.refs))
}

func validRange(o *Ops, start, end PC) bool {
	return validPC(o, start) && validPC(o, end) && start.data <= end.data && start.refs <= end.refs
}

func (pc PC) Add(op OpType) PC {
	size, numRefs := op.props()
	return PC{
		data: pc.data + size,
		refs: pc.refs + numRefs,
	}
}

// Reset start reading from the beginning of ops.
func (r *Reader) Reset(ops *Ops) {
	r.ResetAt(ops, PC{})
}

// ResetAt is like Reset, except it starts reading from pc.
func (r *Reader) ResetAt(ops *Ops, pc PC) {
	r.stack = r.stack[:0]
	Reset(&r.deferOps)
	r.deferDone = false
	r.pc = pc
	r.ops = ops
}

func (r *Reader) Decode() (EncodedOp, bool) {
	if r.ops == nil {
		return EncodedOp{}, false
	}
	deferring := false
	for {
		if len(r.stack) > 0 {
			b := r.stack[len(r.stack)-1]
			if r.pc == b.endPC {
				r.ops = b.ops
				r.pc = b.retPC
				r.stack = r.stack[:len(r.stack)-1]
				continue
			}
		}
		if !validPC(r.ops, r.pc) {
			return EncodedOp{}, false
		}
		data := r.ops.data[r.pc.data:]
		if len(data) == 0 {
			if r.deferDone {
				return EncodedOp{}, false
			}
			r.deferDone = true
			// Execute deferred macros.
			r.ops = &r.deferOps
			r.pc = PC{}
			continue
		}
		key := Key{ops: r.ops, pc: r.pc.data, version: r.ops.version}
		t := OpType(data[0])
		n, nrefs := t.props()
		if n == 0 || n > uint32(len(data)) || r.pc.refs+nrefs > uint32(len(r.ops.refs)) {
			return EncodedOp{}, false
		}
		data = data[:n]
		refs := r.ops.refs[r.pc.refs:]
		refs = refs[:nrefs]
		switch t {
		case TypeDefer:
			deferring = true
			r.pc.data += n
			r.pc.refs += nrefs
			continue
		case TypeAux:
			// An Aux operations is always wrapped in a macro, and
			// its length is the remaining space.
			if len(r.stack) == 0 {
				return EncodedOp{}, false
			}
			block := r.stack[len(r.stack)-1]
			if block.endPC.data < r.pc.data+TypeAuxLen || block.endPC.data > uint32(len(r.ops.data)) {
				return EncodedOp{}, false
			}
			n = block.endPC.data - r.pc.data
			data = data[:n]
		case TypeCall:
			if deferring {
				deferring = false
				// Copy macro for deferred execution.
				if nrefs != 1 {
					panic("internal error: unexpected number of macro refs")
				}
				deferData := Write1(&r.deferOps, int(n), refs[0])
				copy(deferData, data)
				r.pc.data += n
				r.pc.refs += nrefs
				continue
			}
			op, ok := decodeMacroCall(data, refs)
			if !ok || !validRange(op.ops, op.start, op.end) {
				return EncodedOp{}, false
			}
			retPC := r.pc
			retPC.data += n
			retPC.refs += nrefs
			r.stack = append(r.stack, macro{
				ops:   r.ops,
				retPC: retPC,
				endPC: op.end,
			})
			r.ops = op.ops
			r.pc = op.start
			continue
		case TypeMacro:
			op, ok := decodeMacroDefinition(data)
			if !ok {
				return EncodedOp{}, false
			}
			if op.endpc != (PC{}) {
				if !validPC(r.ops, op.endpc) {
					return EncodedOp{}, false
				}
				r.pc = op.endpc
			} else {
				// Treat an incomplete macro as containing all remaining ops.
				r.pc.data = uint32(len(r.ops.data))
				r.pc.refs = uint32(len(r.ops.refs))
			}
			continue
		}
		r.pc.data += n
		r.pc.refs += nrefs
		return EncodedOp{Key: key, Data: data, Refs: refs}, true
	}
}

func decodeMacroDefinition(data []byte) (opMacroDef, bool) {
	if len(data) < TypeMacroLen || OpType(data[0]) != TypeMacro {
		return opMacroDef{}, false
	}
	bo := binary.LittleEndian
	data = data[:TypeMacroLen]
	return opMacroDef{endpc: PC{
		data: bo.Uint32(data[1:]),
		refs: bo.Uint32(data[5:]),
	}}, true
}

func decodeMacroCall(data []byte, refs []any) (macroOp, bool) {
	if len(data) < TypeCallLen || len(refs) < 1 || OpType(data[0]) != TypeCall {
		return macroOp{}, false
	}
	called, ok := refs[0].(*Ops)
	if !ok || called == nil {
		return macroOp{}, false
	}
	bo := binary.LittleEndian
	data = data[:TypeCallLen]
	return macroOp{
		ops: called,
		start: PC{
			data: bo.Uint32(data[1:]),
			refs: bo.Uint32(data[5:]),
		},
		end: PC{
			data: bo.Uint32(data[9:]),
			refs: bo.Uint32(data[13:]),
		},
	}, true
}
