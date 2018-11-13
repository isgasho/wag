// Copyright (c) 2018 Timo Savola. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Documented interfaces.  They are bypassed at runtime.
package isa

import (
	"github.com/tsavola/wag/internal/gen"
	"github.com/tsavola/wag/internal/gen/condition"
	"github.com/tsavola/wag/internal/gen/link"
	"github.com/tsavola/wag/internal/gen/operand"
	"github.com/tsavola/wag/internal/gen/reg"
	"github.com/tsavola/wag/trap"
	"github.com/tsavola/wag/wa"
)

type ISA interface {
	// AlignData writes padding until p.Text.Addr is aligned.
	AlignData(p *gen.Prog, alignment int)

	// AlignFunc writes padding until p.Text.Addr is suitable for a function.
	AlignFunc(p *gen.Prog)

	// PadUntil writes (addr-p.Text.Addr) bytes of padding.
	PadUntil(p *gen.Prog, addr int32)

	// UpdateNearLoad modifies displacement of an instruction generated by
	// MacroAssembler.LoadIntStubNear.  The accessed address is the current
	// text address.
	UpdateNearLoad(text []byte, insnAddr int32)

	// UpdateNearBranch modifies the relocation of a branch instruction.  The
	// branch target is the current text address.
	UpdateNearBranch(text []byte, originAddr int32)

	// UpdateNearBranches modifies relocations of branch instructions.  The
	// branch target is the current text address.
	UpdateNearBranches(text []byte, originAddrs []int32)

	// UpdateFarBranches modifies relocations of branch instructions.
	UpdateFarBranches(text []byte, l *link.L)

	// UpdateStackCheck modifies an instruction generated by
	// MacroAssembler.SetupStackFrame.
	UpdateStackCheck(text []byte, addr int32, depth int)

	// UpdateCalls modifies call instructions, possibly while they are being
	// executed.
	UpdateCalls(text []byte, l *link.L)
}

// MacroAssembler methods by default MUST NOT update condition flags, use
// RegResult or allocate registers.
//
// Methods which return an operand (and are allowed to do such things) may
// return either an allocated register or RegResult.
//
// Some methods which can use RegResult or update condition flags may need to
// still handle them as input operands.
type MacroAssembler interface {
	// AddToStackPtrUpper32 may update condition flags.  It takes ownership of
	// the register.
	AddToStackPtrUpper32(f *gen.Func, r reg.R)

	// Binary may allocate registers, use RegResult and update condition flags.
	Binary(f *gen.Func, props uint16, lhs, rhs operand.O) operand.O

	// Branch may use RegResult and update condition flags.
	Branch(p *gen.Prog, addr int32) int32

	// BranchIfOutOfBounds may use RegResult and update condition flags.  It
	// MUST zero-extend the index register.
	BranchIfOutOfBounds(p *gen.Prog, indexReg reg.R, upperBound, addr int32) int32

	// BranchIfStub may use RegResult and update condition flags.
	BranchIfStub(f *gen.Func, x operand.O, yes, near bool) (sites []int32)

	// BranchIndirect may use RegResult and update condition flags.  It takes
	// ownership of address register, which has already been zero-extended.
	BranchIndirect(f *gen.Func, addr reg.R)

	// Call may use RegResult and update condition flags.
	Call(p *gen.Prog, addr int32) (retAddr int32)

	// CallIndirect may use RegResult and update condition flags.  It takes
	// ownership of funcIndexReg.
	CallIndirect(f *gen.Func, sigIndex int32, funcIndexReg reg.R) int32

	// CallMissing may use RegResult and update condition flags.
	CallMissing(p *gen.Prog) (retAddr int32)

	// ClearIntResultReg may use RegResult and update condition flags.
	ClearIntResultReg(p *gen.Prog)

	// Convert may allocate registers, use RegResult and update condition
	// flags.  The source operand may be RegResult or condition flags.
	Convert(f *gen.Func, props uint16, result wa.Type, source operand.O) operand.O

	// DropStackValues has default restrictions.  The caller will take care of
	// updating the virtual stack pointer.
	DropStackValues(p *gen.Prog, n int)

	// GrowMemory may allocate registers, use RegResult and update condition
	// flags.
	GrowMemory(f *gen.Func, x operand.O) operand.O

	// Init may use RegResult and update condition flags.  It MUST NOT generate
	// over 16 bytes of code.
	Init(p *gen.Prog)

	// InitCallEntry may use RegResult and update condition flags.  It must
	// insert nop instructions until text address is 16-byte aligned.
	InitCallEntry(p *gen.Prog) (retAddr int32)

	// JumpToImportFunc may use RegResult and update condition flags.
	//
	// Void functions must make sure that they don't return any sensitive
	// information in result register.
	JumpToImportFunc(p *gen.Prog, vectorIndex int, variadic bool, argc, sigIndex int)

	// JumpToTrapHandler may use RegResult and update condition flags.  It MUST
	// NOT generate over 16 bytes of code.
	JumpToTrapHandler(p *gen.Prog, id trap.ID)

	// Load may allocate registers, use RegResult and update condition flags.
	// The index operand may be RegResult or the condition flags.
	Load(f *gen.Func, props uint16, index operand.O, result wa.Type, align, offset uint32) operand.O

	// LoadGlobal has default restrictions.
	LoadGlobal(p *gen.Prog, t wa.Type, dest reg.R, offset int32) (zeroExtended bool)

	// LoadIntStubNear may update condition flags.  The register passed as
	// argument is both the index (source) and the destination register.  The
	// index has been zero-extended by the caller.
	LoadIntStubNear(f *gen.Func, index wa.Type, r reg.R) (insnAddr int32)

	// LoadStack has default restrictions.  It MUST zero-extend the (integer)
	// destination register.
	LoadStack(p *gen.Prog, t wa.Type, dest reg.R, offset int32)

	// Move MUST NOT update condition flags unless the operand is the condition
	// flags.  The source operand is consumed.
	Move(f *gen.Func, dest reg.R, x operand.O) (zeroExtended bool)

	// MoveReg has default restrictions.  It MUST zero-extend the (integer)
	// destination register.
	MoveReg(p *gen.Prog, t wa.Type, dest, source reg.R)

	// PushCond has default restrictions.
	PushCond(p *gen.Prog, cond condition.C)

	// PushImm has default restrictions.
	PushImm(p *gen.Prog, value int64)

	// PushReg has default restrictions.
	PushReg(p *gen.Prog, t wa.Type, r reg.R)

	// PushZeros may use RegResult and update condition flags.
	PushZeros(p *gen.Prog, n int)

	// QueryMemorySize may allocate registers, use RegResult and update
	// condition flags.
	QueryMemorySize(f *gen.Func) operand.O

	// Resume may update condition flags.  It MUST NOT generate
	// over 16 bytes of code.
	Resume(p *gen.Prog)

	// Return may use RegResult and update condition flags.
	Return(p *gen.Prog, numStackValues int)

	// Select may allocate registers, use RegResult and update condition flags.
	// The cond operand may be the condition flags.
	Select(f *gen.Func, lhs, rhs, cond operand.O) operand.O

	// SetBool has default restrictions.  It MUST zero-extend the destination
	// register.
	SetBool(p *gen.Prog, dest reg.R, cond condition.C)

	// SetupStackFrame may use RegResult and update condition flags.
	SetupStackFrame(f *gen.Func) (stackCheckAddr int32)

	// Store may allocate registers, use RegResult and update condition flags.
	Store(f *gen.Func, props uint16, index, x operand.O, align, offset uint32)

	// StoreGlobal has default restrictions.
	StoreGlobal(f *gen.Func, offset int32, x operand.O)

	// StoreStack has default restrictions.  The source operand is consumed.
	StoreStack(f *gen.Func, offset int32, x operand.O)

	// StoreStackImm has default restrictions.
	StoreStackImm(p *gen.Prog, t wa.Type, offset int32, value int64)

	// StoreStackReg has default restrictions.
	StoreStackReg(p *gen.Prog, t wa.Type, offset int32, r reg.R)

	// Trap may use RegResult and update condition flags.
	Trap(f *gen.Func, id trap.ID)

	// TrapIfLoopSuspended may use RegResult and update condition flags.
	TrapIfLoopSuspended(f *gen.Func)

	// TrapIfLoopSuspendedSaveInt may update condition flags.
	TrapIfLoopSuspendedSaveInt(f *gen.Func, saveReg reg.R)

	// TrapIfLoopSuspendedElse may use RegResult and update condition flags.
	TrapIfLoopSuspendedElse(f *gen.Func, elseAddr int32)

	// Unary may allocate registers, use RegResult and update condition flags.
	// The operand argument may be RegResult or condition flags.
	Unary(f *gen.Func, props uint16, x operand.O) operand.O

	// ZeroExtendResultReg may use RegResult and update condition flags.
	ZeroExtendResultReg(p *gen.Prog)
}
