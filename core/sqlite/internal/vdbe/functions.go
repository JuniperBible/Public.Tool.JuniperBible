package vdbe

import (
	"fmt"
	"reflect"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/functions"
)

// FunctionContext holds function execution state
type FunctionContext struct {
	registry *functions.Registry
	aggState map[int]*AggregateState // cursor -> aggregate state
}

// AggregateState tracks aggregate function state per cursor
type AggregateState struct {
	funcs  []functions.AggregateFunction
	groups map[string][]functions.AggregateFunction // group key -> funcs
}

// NewFunctionContext creates a new function context with the default registry
func NewFunctionContext() *FunctionContext {
	return &FunctionContext{
		registry: functions.DefaultRegistry(),
		aggState: make(map[int]*AggregateState),
	}
}

// NewFunctionContextWithRegistry creates a function context with a custom registry
func NewFunctionContextWithRegistry(registry *functions.Registry) *FunctionContext {
	return &FunctionContext{
		registry: registry,
		aggState: make(map[int]*AggregateState),
	}
}

// ExecuteFunction runs a scalar function
func (fc *FunctionContext) ExecuteFunction(name string, args []*Mem) (*Mem, error) {
	fn, ok := fc.registry.Lookup(name)
	if !ok {
		return nil, fmt.Errorf("unknown function: %s", name)
	}

	// Check if this is an aggregate function being called as scalar
	if _, isAgg := fn.(functions.AggregateFunction); isAgg {
		return nil, fmt.Errorf("%s() is an aggregate function, cannot be called as scalar", name)
	}

	// Convert Mem to function Values
	values := make([]functions.Value, len(args))
	for i, arg := range args {
		values[i] = memToValue(arg)
	}

	// Call function
	result, err := fn.Call(values)
	if err != nil {
		return nil, err
	}

	// Convert result back to Mem
	return valueToMem(result), nil
}

// GetOrCreateAggregateState gets or creates aggregate state for a cursor
func (fc *FunctionContext) GetOrCreateAggregateState(cursor int) *AggregateState {
	if state, ok := fc.aggState[cursor]; ok {
		return state
	}

	state := &AggregateState{
		funcs:  make([]functions.AggregateFunction, 0),
		groups: make(map[string][]functions.AggregateFunction),
	}
	fc.aggState[cursor] = state
	return state
}

// ResetAggregateState resets aggregate state for a cursor
func (fc *FunctionContext) ResetAggregateState(cursor int) {
	delete(fc.aggState, cursor)
}

// memToValue converts a VDBE Mem to a function Value
func memToValue(m *Mem) functions.Value {
	if m.IsNull() {
		return functions.NewNullValue()
	}
	if m.IsInt() {
		return functions.NewIntValue(m.IntValue())
	}
	if m.IsReal() {
		return functions.NewFloatValue(m.RealValue())
	}
	if m.IsStr() {
		return functions.NewTextValue(m.StrValue())
	}
	if m.IsBlob() {
		return functions.NewBlobValue(m.BlobValue())
	}
	return functions.NewNullValue()
}

// valueToMem converts a function Value to a VDBE Mem
func valueToMem(v functions.Value) *Mem {
	if v.IsNull() {
		return NewMemNull()
	}

	switch v.Type() {
	case functions.TypeInteger:
		return NewMemInt(v.AsInt64())
	case functions.TypeFloat:
		return NewMemReal(v.AsFloat64())
	case functions.TypeText:
		return NewMemStr(v.AsString())
	case functions.TypeBlob:
		return NewMemBlob(v.AsBlob())
	default:
		return NewMemNull()
	}
}

// opFunction implements OP_Function opcode
// P1 = constant mask (bit flags for which args are constant)
// P2 = first argument register
// P3 = output register
// P4 = function name (string)
// P5 = number of arguments
func (v *VDBE) opFunction(p1, p2, p3, p4, p5 int) error {
	// P4 should contain the function name
	instr := v.Program[v.PC-1]
	if instr.P4Type != P4Static && instr.P4Type != P4Dynamic {
		return fmt.Errorf("OP_Function requires function name in P4")
	}

	funcName := instr.P4.Z
	numArgs := p5

	// Collect arguments from registers P2 through P2+numArgs-1
	args := make([]*Mem, numArgs)
	for i := 0; i < numArgs; i++ {
		mem, err := v.GetMem(p2 + i)
		if err != nil {
			return fmt.Errorf("failed to get argument register %d: %w", p2+i, err)
		}
		args[i] = mem
	}

	// Execute the function
	if v.funcCtx == nil {
		v.funcCtx = NewFunctionContext()
	}

	result, err := v.funcCtx.ExecuteFunction(funcName, args)
	if err != nil {
		return fmt.Errorf("function %s failed: %w", funcName, err)
	}

	// Store result in register P3
	dst, err := v.GetMem(p3)
	if err != nil {
		return fmt.Errorf("failed to get result register %d: %w", p3, err)
	}

	return dst.Copy(result)
}

// opAggStep implements OP_AggStep opcode
// P1 = cursor (for grouping context)
// P2 = first argument register
// P3 = aggregate function index
// P4 = function name (string)
// P5 = number of arguments
func (v *VDBE) opAggStep(p1, p2, p3, p4, p5 int) error {
	instr := v.Program[v.PC-1]
	if instr.P4Type != P4Static && instr.P4Type != P4Dynamic {
		return fmt.Errorf("OP_AggStep requires function name in P4")
	}

	funcName := instr.P4.Z
	numArgs := p5
	cursor := p1
	funcIndex := p3

	// Initialize function context if needed
	if v.funcCtx == nil {
		v.funcCtx = NewFunctionContext()
	}

	// Get or create aggregate state
	aggState := v.funcCtx.GetOrCreateAggregateState(cursor)

	// Ensure we have enough function slots
	for len(aggState.funcs) <= funcIndex {
		aggState.funcs = append(aggState.funcs, nil)
	}

	// Create aggregate function instance if needed
	if aggState.funcs[funcIndex] == nil {
		fn, ok := v.funcCtx.registry.Lookup(funcName)
		if !ok {
			return fmt.Errorf("unknown aggregate function: %s", funcName)
		}

		aggFn, ok := fn.(functions.AggregateFunction)
		if !ok {
			return fmt.Errorf("%s is not an aggregate function", funcName)
		}

		// Create a new instance using reflection
		aggFn = createAggregateInstance(aggFn)
		aggState.funcs[funcIndex] = aggFn
	}

	// Collect arguments from registers
	args := make([]*Mem, numArgs)
	for i := 0; i < numArgs; i++ {
		mem, err := v.GetMem(p2 + i)
		if err != nil {
			return fmt.Errorf("failed to get argument register %d: %w", p2+i, err)
		}
		args[i] = mem
	}

	// Convert to function values
	values := make([]functions.Value, numArgs)
	for i, arg := range args {
		values[i] = memToValue(arg)
	}

	// Execute step function
	aggFn := aggState.funcs[funcIndex]
	if err := aggFn.Step(values); err != nil {
		return fmt.Errorf("aggregate step failed for %s: %w", funcName, err)
	}

	return nil
}

// opAggFinal implements OP_AggFinal opcode
// P1 = cursor (for grouping context)
// P2 = output register
// P3 = aggregate function index
func (v *VDBE) opAggFinal(p1, p2, p3 int) error {
	cursor := p1
	outputReg := p2
	funcIndex := p3

	if v.funcCtx == nil {
		return fmt.Errorf("no function context available")
	}

	aggState := v.funcCtx.GetOrCreateAggregateState(cursor)

	// Check if we have the function
	if funcIndex >= len(aggState.funcs) || aggState.funcs[funcIndex] == nil {
		return fmt.Errorf("no aggregate function at index %d", funcIndex)
	}

	// Finalize the aggregate
	aggFn := aggState.funcs[funcIndex]
	result, err := aggFn.Final()
	if err != nil {
		return fmt.Errorf("aggregate finalization failed: %w", err)
	}

	// Store result in output register
	dst, err := v.GetMem(outputReg)
	if err != nil {
		return fmt.Errorf("failed to get result register %d: %w", outputReg, err)
	}

	resultMem := valueToMem(result)
	return dst.Copy(resultMem)
}

// createAggregateInstance creates a new instance of an aggregate function
// This is needed because aggregate functions maintain state and we need
// a fresh instance for each execution context
func createAggregateInstance(fn functions.AggregateFunction) functions.AggregateFunction {
	// Use reflection to create a new instance
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() == reflect.Ptr {
		fnType = fnType.Elem()
	}

	newInstance := reflect.New(fnType).Interface()
	if aggFn, ok := newInstance.(functions.AggregateFunction); ok {
		return aggFn
	}

	// Fallback: return the original (not ideal but safe)
	fn.Reset()
	return fn
}
