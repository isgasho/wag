package values

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/tsavola/wag/internal/regs"
	"github.com/tsavola/wag/internal/types"
)

type Storage int

const (
	Nowhere = Storage(iota)
	Imm
	ROData
	RegVar
	RegTemp
	StackVar
	StackPop
	ConditionFlags
)

var (
	NoOperand       = Operand{Storage: Nowhere}
	StackPopOperand = Operand{Storage: StackPop}
)

type Condition int

const (
	EQ = Condition(iota)
	NE
	GE_S
	GT_S
	GE_U
	GT_U
	LE_S
	LT_S
	LE_U
	LT_U
)

var InvertedConditions = []Condition{
	NE,   // EQ
	EQ,   // NE
	LT_S, // GE_S
	LE_S, // GT_S
	LT_U, // GE_U
	LE_U, // GT_U
	GT_S, // LE_S
	GE_S, // LT_U
	GT_U, // LE_U
	GE_U, // LT_U
}

type Operand struct {
	Storage Storage
	X       uint64
}

func ImmOperand(t types.T, value int) Operand {
	var x uint64

	switch t.Size() {
	case types.Size32:
		x = uint64(uint32(int32(value)))

	case types.Size64:
		x = uint64(int64(value))

	default:
		panic(t)
	}

	return Operand{Imm, x}
}

func RODataOperand(addr int) Operand {
	return Operand{ROData, uint64(addr)}
}

func RegTempOperand(reg regs.R) Operand {
	return Operand{RegTemp, uint64(byte(reg))}
}

func RegVarOperand(reg regs.R) Operand {
	return Operand{RegVar, uint64(byte(reg))}
}

func StackVarOperand(offset int) Operand {
	return Operand{StackVar, uint64(offset)}
}

func ConditionFlagsOperand(cond Condition) Operand {
	return Operand{ConditionFlags, uint64(int(cond))}
}

// Pure operands don't need to be saved during arbitrary expression evaluation.
func (o Operand) Pure() (ok bool) {
	switch o.Storage {
	case Nowhere, Imm, ROData, StackPop:
		ok = true
	}
	return
}

// Once operands can be accessed only once.
func (o Operand) Once() bool {
	return o.Storage == StackPop
}

func (o Operand) Imm(t types.T) (imm interface{}) {
	imm, ok := o.CheckImm(t)
	if !ok {
		panic(o)
	}
	return
}

func (o Operand) CheckImm(t types.T) (imm interface{}, ok bool) {
	if o.Storage != Imm {
		return
	}

	switch t.Size() {
	case types.Size32:
		imm = uint32(o.X)

	case types.Size64:
		imm = o.X

	default:
		panic(t)
	}

	ok = true
	return
}

func (o Operand) ImmValue(t types.T) (value int64) {
	value, ok := o.CheckImmValue(t)
	if !ok {
		panic(o)
	}
	return
}

func (o Operand) CheckImmValue(t types.T) (value int64, ok bool) {
	if o.Storage != Imm {
		return
	}

	switch t.Size() {
	case types.Size32:
		value = int64(int32(uint32(o.X)))

	case types.Size64:
		value = int64(o.X)

	default:
		panic(t)
	}

	ok = true
	return
}

func (o Operand) Addr() (addr int) {
	addr, ok := o.CheckROData()
	if !ok {
		panic(o)
	}
	return
}

func (o Operand) CheckROData() (addr int, ok bool) {
	if o.Storage != ROData {
		return
	}

	if o.X >= 0x80000000-8 {
		panic(o)
	}

	addr = int(o.X)
	ok = true
	return
}

func (o Operand) Reg() (reg regs.R) {
	reg, ok := o.CheckAnyReg()
	if !ok {
		panic(o)
	}
	return
}

func (o Operand) CheckAnyReg() (reg regs.R, ok bool) {
	switch o.Storage {
	case RegVar, RegTemp:
		reg = regs.R(byte(o.X))
		ok = true
	}
	return
}

func (o Operand) CheckRegVar() (reg regs.R, ok bool) {
	if o.Storage == RegVar {
		reg = regs.R(byte(o.X))
		ok = true
	}
	return
}

func (o Operand) CheckRegTemp() (reg regs.R, ok bool) {
	if o.Storage == RegTemp {
		reg = regs.R(byte(o.X))
		ok = true
	}
	return
}

func (o Operand) Offset() (offset int) {
	offset, ok := o.CheckStackVar()
	if !ok {
		panic(o)
	}
	return
}

func (o Operand) CheckStackVar() (offset int, ok bool) {
	if o.Storage != StackVar {
		return
	}

	value := int64(o.X)
	if value < -0x80000000 || value >= 0x80000000 {
		panic(value)
	}

	offset = int(value)
	ok = true
	return
}

func (o Operand) Condition() (cond Condition) {
	cond, ok := o.CheckConditionFlags()
	if !ok {
		panic(o)
	}
	return
}

func (o Operand) CheckConditionFlags() (cond Condition, ok bool) {
	if o.Storage == ConditionFlags {
		cond = Condition(int(o.X))
		ok = true
	}
	return
}

func (o Operand) String() string {
	switch o.Storage {
	case Nowhere:
		return "nowhere"

	case Imm:
		return fmt.Sprintf("immediate data 0x%x", o.X)

	case ROData:
		return fmt.Sprintf("in read-only data at offset 0x%x", o.X)

	case RegVar:
		return fmt.Sprintf("cached in register #%d", o.X)

	case RegTemp:
		return fmt.Sprintf("temporarily in register #%d", o.X)

	case StackVar:
		return fmt.Sprintf("on stack at offset 0x%x", o.X)

	case StackPop:
		return "pushed on stack"

	case ConditionFlags:
		return "in CPU condition flags"

	default:
		return "corrupted"
	}
}

func ParseImm(t types.T, x interface{}) Operand {
	var value uint64

	switch t {
	case types.I32:
		value = ParseI32(x)

	case types.I64:
		value = ParseI64(x)

	case types.F32:
		value = ParseF32(x)

	case types.F64:
		value = ParseF64(x)

	default:
		panic(t)
	}

	return Operand{Imm, value}
}

func ParseI32(x interface{}) uint64 {
	s := nonOctalize(x.(string))

	signed64, err := strconv.ParseInt(s, 0, 32)
	if err == nil {
		return uint64(signed64)
	}

	unsigned64, err := strconv.ParseUint(s, 0, 32)
	if err == nil {
		return unsigned64
	}

	panic(err)
}

func ParseI64(x interface{}) uint64 {
	s := nonOctalize(x.(string))

	signed64, err := strconv.ParseInt(s, 0, 64)
	if err == nil {
		return uint64(signed64)
	}

	unsigned64, err := strconv.ParseUint(s, 0, 64)
	if err == nil {
		return unsigned64
	}

	panic(err)
}

func ParseF32(x interface{}) uint64 {
	s := x.(string)

	value64, err := strconv.ParseFloat(s, 32)
	if err == nil {
		return uint64(math.Float32bits(float32(value64)))
	}

	panic(err)
}

func ParseF64(x interface{}) uint64 {
	s := x.(string)

	value64, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return math.Float64bits(value64)
	}

	panic(err)
}

func nonOctalize(s string) string {
	for len(s) > 1 && strings.HasPrefix(s, "0") && !strings.HasPrefix(s, "0x") {
		s = s[1:]
	}

	return s
}
