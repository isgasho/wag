// Copyright (c) 2016 Timo Savola. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x86

import (
	"github.com/tsavola/wag/abi"
	"github.com/tsavola/wag/internal/gen"
	"github.com/tsavola/wag/internal/regs"
)

type prefix interface {
	put(text *gen.Text, t abi.Type, ro, index, rmOrBase byte)
}

type constPrefix []byte

func (bytes constPrefix) put(text *gen.Text, t abi.Type, ro, index, rmOrBase byte) {
	text.PutBytes(bytes)
}

type multiPrefix []prefix

func (array multiPrefix) put(text *gen.Text, t abi.Type, ro, index, rmOrBase byte) {
	for _, p := range array {
		p.put(text, t, ro, index, rmOrBase)
	}
}

const (
	Rex  = (1 << 6)
	RexW = Rex | (1 << 3)
	RexR = Rex | (1 << 2)
	RexX = Rex | (1 << 1)
	RexB = Rex | (1 << 0)
)

func putRex(text *gen.Text, rex, ro, index, rmOrBase byte) {
	if ro >= 8 {
		rex |= RexR
	}
	if index >= 8 {
		rex |= RexX
	}
	if rmOrBase >= 8 {
		rex |= RexB
	}

	if rex != 0 {
		text.PutByte(rex)
	}
}

func putRexSize(text *gen.Text, t abi.Type, ro, index, rmOrBase byte) {
	var rex byte

	switch t.Size() {
	case abi.Size32:

	case abi.Size64:
		rex |= RexW

	default:
		panic(t)
	}

	putRex(text, rex, ro, index, rmOrBase)
}

type mod byte

const (
	ModMem       = mod(0)
	ModMemDisp8  = mod((0 << 7) | (1 << 6))
	ModMemDisp32 = mod((1 << 7) | (0 << 6))
	ModReg       = mod((1 << 7) | (1 << 6))
)

func dispMod(t abi.Type, baseReg regs.R, offset int32) mod {
	switch {
	case offset == 0 && (baseReg&7) != 0x5: // rbp and r13 need displacement
		return ModMem

	case offset >= -0x80 && offset < 0x80:
		return ModMemDisp8

	default:
		return ModMemDisp32
	}
}

func putMod(text *gen.Text, mod mod, ro, rm byte) {
	text.PutByte(byte(mod) | ((ro & 7) << 3) | (rm & 7))
}

func putDisp(text *gen.Text, mod mod, offset int32) {
	switch mod {
	case ModMemDisp8:
		gen.PutInt8(text, int8(offset))

	case ModMemDisp32:
		gen.PutInt32(text, offset)
	}
}

const (
	MemSIB    = byte((1 << 2))
	MemDisp32 = byte((1 << 2) | (1 << 0))
)

const (
	NoIndex = regs.R((1 << 2))
	NoBase  = regs.R((1 << 2) | (1 << 0))
)

func putSib(text *gen.Text, scale byte, index, base regs.R) {
	if scale >= 4 {
		panic("scale factor out of bounds")
	}

	text.PutByte((scale << 6) | (byte(index&7) << 3) | byte(base&7))
}

//
type insnConst []byte

func (opcode insnConst) op(text *gen.Text) {
	text.PutBytes(opcode)
}

//
type insnO struct {
	opbase byte
}

func (i insnO) op(text *gen.Text, reg regs.R) {
	if reg >= 8 {
		panic("register not supported by instruction")
	}

	text.PutByte(i.opbase + byte(reg))
}

//
type insnI []byte

func (opcode insnI) op8(text *gen.Text, value int8) {
	text.PutBytes(opcode)
	gen.PutInt8(text, value)
}

func (opcode insnI) op32(text *gen.Text, value int32) {
	text.PutBytes(opcode)
	gen.PutInt32(text, value)
}

//
type insnAddr8 []byte

func (opcode insnAddr8) size() int32 {
	return int32(len(opcode)) + 1
}

func (opcode insnAddr8) op(text *gen.Text, addr int32) (ok bool) {
	insnSize := int32(len(opcode)) + 1
	siteAddr := text.Pos() + insnSize
	offset := addr - siteAddr

	if offset >= -0x80 && offset < 0x80 {
		text.PutBytes(opcode)
		gen.PutInt8(text, int8(offset))
		ok = true
	}
	return
}

func (i insnAddr8) opStub(text *gen.Text) {
	i.op(text, text.Pos()) // infinite loop as placeholder
}

//
type insnAddr32 []byte

func (opcode insnAddr32) size() int32 {
	return int32(len(opcode)) + 4
}

func (i insnAddr32) op(text *gen.Text, addr int32) {
	var offset int32
	if addr != 0 {
		siteAddr := text.Pos() + i.size()
		offset = addr - siteAddr
	} else {
		offset = -i.size() // infinite loop as placeholder
	}
	i.put(text, offset)
}

func (i insnAddr32) opMissingFunc(text *gen.Text) {
	siteAddr := text.Pos() + i.size()
	i.put(text, -siteAddr)
}

func (opcode insnAddr32) put(text *gen.Text, offset int32) {
	text.PutBytes(opcode)
	gen.PutInt32(text, offset)
}

//
type insnAddr struct {
	rel8  insnAddr8
	rel32 insnAddr32
}

func (i insnAddr) op(text *gen.Text, addr int32) {
	var ok bool
	if addr != 0 {
		ok = i.rel8.op(text, addr)
	}
	if !ok {
		i.rel32.op(text, addr)
	}
}

//
type insnRex []byte

func (opcode insnRex) op(text *gen.Text, t abi.Type) {
	putRexSize(text, t, 0, 0, 0)
	text.PutBytes(opcode)
}

//
type insnRexOM struct {
	opcode []byte
	ro     byte
}

func (i insnRexOM) opReg(text *gen.Text, reg regs.R) {
	putRex(text, 0, 0, 0, byte(reg))
	text.PutBytes(i.opcode)
	putMod(text, ModReg, i.ro, byte(reg))
}

//
type insnRexO struct {
	opbase byte
}

func (i insnRexO) op(text *gen.Text, t abi.Type, reg regs.R) {
	putRexSize(text, t, 0, 0, byte(reg))
	text.PutByte(i.opbase + (byte(reg) & 7))
}

//
type insnRexOI struct {
	opbase byte
}

func (i insnRexOI) op32(text *gen.Text, t abi.Type, reg regs.R, value uint32) {
	putRexSize(text, t, 0, 0, byte(reg))
	text.PutByte(i.opbase + (byte(reg) & 7))
	gen.PutInt32(text, int32(value))
}

func (i insnRexOI) op64(text *gen.Text, t abi.Type, reg regs.R, value int64) {
	putRexSize(text, t, 0, 0, byte(reg))
	text.PutByte(i.opbase + (byte(reg) & 7))
	gen.PutInt64(text, value)
}

//
type insnRexM struct {
	opcode []byte
	ro     byte
}

func (i insnRexM) opReg(text *gen.Text, t abi.Type, reg regs.R) {
	putRexSize(text, t, 0, 0, byte(reg))
	text.PutBytes(i.opcode)
	putMod(text, ModReg, i.ro, byte(reg))
}

func (i insnRexM) opIndirect(text *gen.Text, t abi.Type, reg regs.R, disp int32) {
	mod := dispMod(t, reg, disp)

	putRexSize(text, t, 0, 0, byte(reg))
	text.PutBytes(i.opcode)

	if reg != 12 {
		putMod(text, mod, i.ro, byte(reg))
	} else {
		putMod(text, mod, i.ro, MemSIB)
		putSib(text, 0, NoIndex, reg)
	}

	putDisp(text, mod, disp)
}

func (i insnRexM) opStack(text *gen.Text, t abi.Type, disp int32) {
	mod := dispMod(t, RegStackPtr, disp)

	putRexSize(text, t, 0, 0, 0)
	text.PutBytes(i.opcode)
	putMod(text, mod, i.ro, MemSIB)
	putSib(text, 0, RegStackPtr, RegStackPtr)
	putDisp(text, mod, disp)
}

var (
	noRexMInsn = insnRexM{nil, 0}
)

//
type insnPrefix struct {
	prefix   prefix
	opcodeRM []byte
	opcodeMR []byte
}

func (i insnPrefix) opFromReg(text *gen.Text, t abi.Type, target, source regs.R) {
	putPrefixRegInsn(text, i.prefix, t, i.opcodeRM, byte(target), byte(source))
}

func (i insnPrefix) opFromAddr(text *gen.Text, t abi.Type, target regs.R, scale uint8, index regs.R, addr int32) {
	putPrefixAddrInsn(text, i.prefix, t, i.opcodeRM, target, scale, index, addr)
}

func (i insnPrefix) opFromIndirect(text *gen.Text, t abi.Type, target regs.R, scale uint8, index, base regs.R, disp int32) {
	putPrefixIndirectInsn(text, i.prefix, t, i.opcodeRM, target, scale, index, base, disp)
}

func (i insnPrefix) opFromStack(text *gen.Text, t abi.Type, target regs.R, disp int32) {
	putPrefixStackInsn(text, i.prefix, t, i.opcodeRM, target, disp)
}

func (i insnPrefix) opToReg(text *gen.Text, t abi.Type, target, source regs.R) {
	putPrefixRegInsn(text, i.prefix, t, i.opcodeMR, byte(source), byte(target))
}

func (i insnPrefix) opToAddr(text *gen.Text, t abi.Type, source regs.R, scale uint8, index regs.R, addr int32) {
	putPrefixAddrInsn(text, i.prefix, t, i.opcodeMR, source, scale, index, addr)
}

func (i insnPrefix) opToIndirect(text *gen.Text, t abi.Type, target regs.R, scale uint8, index, base regs.R, disp int32) {
	putPrefixIndirectInsn(text, i.prefix, t, i.opcodeMR, target, scale, index, base, disp)
}

func (i insnPrefix) opToStack(text *gen.Text, t abi.Type, source regs.R, disp int32) {
	putPrefixStackInsn(text, i.prefix, t, i.opcodeMR, source, disp)
}

func putPrefixRegInsn(text *gen.Text, p prefix, t abi.Type, opcode []byte, ro, rm byte) {
	if opcode == nil {
		panic("instruction not supported")
	}

	p.put(text, t, ro, 0, rm)
	text.PutBytes(opcode)
	putMod(text, ModReg, ro, rm)
}

func putPrefixAddrInsn(text *gen.Text, p prefix, t abi.Type, opcode []byte, reg regs.R, scale uint8, index regs.R, addr int32) {
	if opcode == nil {
		panic("instruction not supported")
	}

	p.put(text, t, byte(reg), 0, 0)
	text.PutBytes(opcode)
	putMod(text, ModMem, byte(reg), MemSIB)
	putSib(text, scale, index, NoBase)
	gen.PutInt32(text, addr)
}

func putPrefixIndirectInsn(text *gen.Text, p prefix, t abi.Type, opcode []byte, reg regs.R, scale uint8, index, base regs.R, disp int32) {
	if opcode == nil {
		panic("instruction not supported")
	}

	mod := dispMod(t, base, disp)

	p.put(text, t, byte(reg), byte(index), byte(base))
	text.PutBytes(opcode)

	if scale == 0 && index == NoIndex && base != 12 {
		putMod(text, mod, byte(reg), byte(base))
	} else {
		putMod(text, mod, byte(reg), MemSIB)
		putSib(text, scale, index, base)
	}

	putDisp(text, mod, disp)
}

func putPrefixStackInsn(text *gen.Text, p prefix, t abi.Type, opcode []byte, reg regs.R, disp int32) {
	mod := dispMod(t, RegStackPtr, disp)

	p.put(text, t, byte(reg), 0, 0)
	text.PutBytes(opcode)
	putMod(text, mod, byte(reg), MemSIB)
	putSib(text, 0, RegStackPtr, RegStackPtr)
	putDisp(text, mod, disp)
}

//
type insnPrefixRexRM struct {
	prefix prefix
	opcode []byte
}

func (i insnPrefixRexRM) opReg(text *gen.Text, floatType, intType abi.Type, target, source regs.R) {
	i.prefix.put(text, floatType, 0, 0, 0)
	putRexSize(text, intType, byte(target), 0, byte(source))
	text.PutBytes(i.opcode)
	putMod(text, ModReg, byte(target), byte(source))
}

//
type insnPrefixMI struct {
	prefix   prefix
	opcode8  byte
	opcode16 byte
	opcode32 byte
	ro       byte
}

func (i insnPrefixMI) opImm(text *gen.Text, t abi.Type, reg regs.R, value int32) {
	opcode := i.immOpcode(value)

	i.prefix.put(text, t, 0, 0, byte(reg))
	text.PutByte(opcode)
	putMod(text, ModReg, i.ro, byte(reg))
	i.putImm(text, opcode, value)
}

func (i insnPrefixMI) opImm8(text *gen.Text, t abi.Type, reg regs.R, value uint8) {
	i.prefix.put(text, t, 0, 0, byte(reg))
	text.PutByte(i.opcode8)
	putMod(text, ModReg, i.ro, byte(reg))
	text.PutByte(value)
}

func (i insnPrefixMI) opImmToIndirect(text *gen.Text, t abi.Type, scale uint8, index, base regs.R, disp, value int32) {
	mod := dispMod(t, base, disp)
	opcode := i.immOpcode(value)

	i.prefix.put(text, t, 0, byte(index), byte(base))
	text.PutByte(opcode)

	if scale == 0 && index == NoIndex && base != 12 {
		putMod(text, mod, i.ro, byte(base))
	} else {
		putMod(text, mod, i.ro, MemSIB)
		putSib(text, scale, index, base)
	}

	putDisp(text, mod, disp)
	i.putImm(text, opcode, value)
}

func (i insnPrefixMI) opImmToStack(text *gen.Text, t abi.Type, disp, value int32) {
	mod := dispMod(t, RegStackPtr, disp)
	opcode := i.immOpcode(value)

	i.prefix.put(text, t, 0, 0, 0)
	text.PutByte(opcode)
	putMod(text, mod, i.ro, MemSIB)
	putSib(text, 0, RegStackPtr, RegStackPtr)
	putDisp(text, mod, disp)
	i.putImm(text, opcode, value)
}

func (i insnPrefixMI) immOpcode(value int32) byte {
	switch {
	case i.opcode8 != 0 && value >= -0x80 && value < 0x80:
		return i.opcode8

	case i.opcode16 != 0 && value >= -0x8000 && value < 0x8000:
		return i.opcode16

	case i.opcode32 != 0:
		return i.opcode32

	default:
		panic("immediate value out of range")
	}
}

func (i insnPrefixMI) putImm(text *gen.Text, opcode byte, value int32) {
	switch opcode {
	case i.opcode8:
		gen.PutInt8(text, int8(value))

	case i.opcode16:
		gen.PutInt16(text, int16(value))

	default: // i.opcode32
		gen.PutInt32(text, value)
	}
}

var (
	noPrefixMIInsn = insnPrefixMI{nil, 0, 0, 0, 0}
)

//
type insnSuffixRMI struct {
	opcode []byte
	suffix prefix
}

func (i insnSuffixRMI) opReg(text *gen.Text, t abi.Type, target, source regs.R, value int8) {
	text.PutBytes(i.opcode)
	i.suffix.put(text, t, byte(target), 0, byte(source))
	putMod(text, ModReg, byte(target), byte(source))
	gen.PutInt8(text, value)
}

//
type binaryInsn struct {
	insnPrefix
	insnPrefixMI
}

//
type pushPopInsn struct {
	regLow insnO
	regAny insnRexM
}

func (i pushPopInsn) op(text *gen.Text, reg regs.R) {
	if reg < 8 {
		i.regLow.op(text, reg)
	} else {
		i.regAny.opReg(text, abi.I32, reg)
	}
}

//
type xchgInsn struct {
	r0 insnRexO
	insnPrefix
}

func (i xchgInsn) opFromReg(text *gen.Text, t abi.Type, a, b regs.R) {
	switch {
	case a == regs.R(0):
		i.r0.op(text, t, b)

	case b == regs.R(0):
		i.r0.op(text, t, a)

	default:
		i.insnPrefix.opFromReg(text, t, a, b)
	}
}

//
type shiftImmInsn struct {
	one insnRexM
	any insnPrefixMI
}

func (i shiftImmInsn) defined() bool {
	return i.one.opcode != nil
}

func (i shiftImmInsn) op(text *gen.Text, t abi.Type, reg regs.R, value uint8) {
	if value == 1 {
		i.one.opReg(text, t, reg)
	} else {
		i.any.opImm8(text, t, reg, value)
	}
}

var (
	noShiftImmInsn = shiftImmInsn{noRexMInsn, noPrefixMIInsn}
)

//
type movImmInsn struct {
	imm32 insnPrefixMI
	imm   insnRexOI
}

func (i movImmInsn) op(text *gen.Text, t abi.Type, reg regs.R, value int64) {
	switch {
	case value >= -0x80000000 && value < 0x80000000:
		i.imm32.opImm(text, t, reg, int32(value))

	case t.Size() == abi.Size64 && value >= 0 && value < 0x100000000:
		i.imm.op32(text, abi.I32, reg, uint32(value))

	default:
		i.imm.op64(text, t, reg, value)
	}
}
