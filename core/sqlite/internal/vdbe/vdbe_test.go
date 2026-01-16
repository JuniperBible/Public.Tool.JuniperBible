package vdbe

import (
	"testing"
)

func TestMemBasicTypes(t *testing.T) {
	t.Run("Null", func(t *testing.T) {
		mem := NewMemNull()
		if !mem.IsNull() {
			t.Error("Expected NULL flag to be set")
		}
		if mem.IntValue() != 0 {
			t.Error("NULL should convert to 0 integer")
		}
	})

	t.Run("Integer", func(t *testing.T) {
		mem := NewMemInt(42)
		if !mem.IsInt() {
			t.Error("Expected INT flag to be set")
		}
		if mem.IntValue() != 42 {
			t.Errorf("Expected 42, got %d", mem.IntValue())
		}
		if mem.StrValue() != "42" {
			t.Errorf("Expected '42', got '%s'", mem.StrValue())
		}
	})

	t.Run("Real", func(t *testing.T) {
		mem := NewMemReal(3.14159)
		if !mem.IsReal() {
			t.Error("Expected REAL flag to be set")
		}
		if mem.RealValue() != 3.14159 {
			t.Errorf("Expected 3.14159, got %f", mem.RealValue())
		}
	})

	t.Run("String", func(t *testing.T) {
		mem := NewMemStr("hello")
		if !mem.IsStr() {
			t.Error("Expected STR flag to be set")
		}
		if mem.StrValue() != "hello" {
			t.Errorf("Expected 'hello', got '%s'", mem.StrValue())
		}
	})

	t.Run("Blob", func(t *testing.T) {
		data := []byte{1, 2, 3, 4, 5}
		mem := NewMemBlob(data)
		if !mem.IsBlob() {
			t.Error("Expected BLOB flag to be set")
		}
		blob := mem.BlobValue()
		if len(blob) != 5 {
			t.Errorf("Expected blob length 5, got %d", len(blob))
		}
	})
}

func TestMemConversions(t *testing.T) {
	t.Run("IntToReal", func(t *testing.T) {
		mem := NewMemInt(42)
		if err := mem.Realify(); err != nil {
			t.Fatalf("Realify failed: %v", err)
		}
		if !mem.IsReal() {
			t.Error("Expected REAL flag after conversion")
		}
		if mem.RealValue() != 42.0 {
			t.Errorf("Expected 42.0, got %f", mem.RealValue())
		}
	})

	t.Run("StringToInt", func(t *testing.T) {
		mem := NewMemStr("123")
		if err := mem.Integerify(); err != nil {
			t.Fatalf("Integerify failed: %v", err)
		}
		if mem.IntValue() != 123 {
			t.Errorf("Expected 123, got %d", mem.IntValue())
		}
	})

	t.Run("StringToReal", func(t *testing.T) {
		mem := NewMemStr("3.14")
		if err := mem.Realify(); err != nil {
			t.Fatalf("Realify failed: %v", err)
		}
		if mem.RealValue() != 3.14 {
			t.Errorf("Expected 3.14, got %f", mem.RealValue())
		}
	})

	t.Run("IntToString", func(t *testing.T) {
		mem := NewMemInt(42)
		if err := mem.Stringify(); err != nil {
			t.Fatalf("Stringify failed: %v", err)
		}
		if mem.StrValue() != "42" {
			t.Errorf("Expected '42', got '%s'", mem.StrValue())
		}
	})
}

func TestMemArithmetic(t *testing.T) {
	t.Run("AddIntegers", func(t *testing.T) {
		a := NewMemInt(10)
		b := NewMemInt(20)
		if err := a.Add(b); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if a.IntValue() != 30 {
			t.Errorf("Expected 30, got %d", a.IntValue())
		}
	})

	t.Run("SubtractIntegers", func(t *testing.T) {
		a := NewMemInt(50)
		b := NewMemInt(20)
		if err := a.Subtract(b); err != nil {
			t.Fatalf("Subtract failed: %v", err)
		}
		if a.IntValue() != 30 {
			t.Errorf("Expected 30, got %d", a.IntValue())
		}
	})

	t.Run("MultiplyIntegers", func(t *testing.T) {
		a := NewMemInt(6)
		b := NewMemInt(7)
		if err := a.Multiply(b); err != nil {
			t.Fatalf("Multiply failed: %v", err)
		}
		if a.IntValue() != 42 {
			t.Errorf("Expected 42, got %d", a.IntValue())
		}
	})

	t.Run("DivideReals", func(t *testing.T) {
		a := NewMemReal(10.0)
		b := NewMemReal(4.0)
		if err := a.Divide(b); err != nil {
			t.Fatalf("Divide failed: %v", err)
		}
		if a.RealValue() != 2.5 {
			t.Errorf("Expected 2.5, got %f", a.RealValue())
		}
	})

	t.Run("DivideByZero", func(t *testing.T) {
		a := NewMemInt(10)
		b := NewMemInt(0)
		if err := a.Divide(b); err != nil {
			t.Fatalf("Divide failed: %v", err)
		}
		if !a.IsNull() {
			t.Error("Expected NULL after division by zero")
		}
	})

	t.Run("Remainder", func(t *testing.T) {
		a := NewMemInt(17)
		b := NewMemInt(5)
		if err := a.Remainder(b); err != nil {
			t.Fatalf("Remainder failed: %v", err)
		}
		if a.IntValue() != 2 {
			t.Errorf("Expected 2, got %d", a.IntValue())
		}
	})
}

func TestMemComparison(t *testing.T) {
	t.Run("IntegerComparison", func(t *testing.T) {
		a := NewMemInt(10)
		b := NewMemInt(20)
		c := NewMemInt(10)

		if a.Compare(b) != -1 {
			t.Error("10 should be less than 20")
		}
		if b.Compare(a) != 1 {
			t.Error("20 should be greater than 10")
		}
		if a.Compare(c) != 0 {
			t.Error("10 should equal 10")
		}
	})

	t.Run("StringComparison", func(t *testing.T) {
		a := NewMemStr("apple")
		b := NewMemStr("banana")
		c := NewMemStr("apple")

		if a.Compare(b) != -1 {
			t.Error("'apple' should be less than 'banana'")
		}
		if b.Compare(a) != 1 {
			t.Error("'banana' should be greater than 'apple'")
		}
		if a.Compare(c) != 0 {
			t.Error("'apple' should equal 'apple'")
		}
	})

	t.Run("NullComparison", func(t *testing.T) {
		a := NewMemNull()
		b := NewMemNull()
		c := NewMemInt(42)

		if a.Compare(b) != 0 {
			t.Error("NULL should equal NULL")
		}
		if a.Compare(c) != -1 {
			t.Error("NULL should be less than any value")
		}
	})
}

func TestMemCopyMove(t *testing.T) {
	t.Run("DeepCopy", func(t *testing.T) {
		src := NewMemStr("hello")
		dst := NewMem()
		if err := dst.Copy(src); err != nil {
			t.Fatalf("Copy failed: %v", err)
		}

		if dst.StrValue() != "hello" {
			t.Error("Copy didn't preserve string value")
		}

		// Modify source shouldn't affect destination
		src.SetStr("world")
		if dst.StrValue() != "hello" {
			t.Error("Deep copy was affected by source modification")
		}
	})

	t.Run("Move", func(t *testing.T) {
		src := NewMemInt(42)
		dst := NewMem()
		dst.Move(src)

		if dst.IntValue() != 42 {
			t.Error("Move didn't transfer value")
		}
		if src.flags != MemUndefined {
			t.Errorf("Source should be undefined after move, got flags=%v", src.flags)
		}
	})

	t.Run("ShallowCopy", func(t *testing.T) {
		src := NewMemStr("test")
		dst := NewMem()
		dst.ShallowCopy(src)

		if dst.StrValue() != "test" {
			t.Error("Shallow copy didn't preserve value")
		}
	})
}

func TestVdbeBasicExecution(t *testing.T) {
	t.Run("SimpleProgram", func(t *testing.T) {
		v := New()
		v.AllocMemory(10)

		// Program: r[1] = 42; Halt
		v.AddOp(OpInteger, 42, 1, 0)
		v.AddOp(OpHalt, 0, 0, 0)

		if err := v.Run(); err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		mem, _ := v.GetMem(1)
		if mem.IntValue() != 42 {
			t.Errorf("Expected 42, got %d", mem.IntValue())
		}
	})

	t.Run("ArithmeticProgram", func(t *testing.T) {
		v := New()
		v.AllocMemory(10)

		// Program: r[1] = 10; r[2] = 20; r[3] = r[1] + r[2]; Halt
		v.AddOp(OpInteger, 10, 1, 0)
		v.AddOp(OpInteger, 20, 2, 0)
		v.AddOp(OpAdd, 1, 2, 3)
		v.AddOp(OpHalt, 0, 0, 0)

		if err := v.Run(); err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		mem, _ := v.GetMem(3)
		if mem.IntValue() != 30 {
			t.Errorf("Expected 30, got %d", mem.IntValue())
		}
	})

	t.Run("ConditionalJump", func(t *testing.T) {
		v := New()
		v.AllocMemory(10)

		// Program: r[1] = 1; if r[1] goto 4; r[2] = 99; r[2] = 42; Halt
		v.AddOp(OpInteger, 1, 1, 0)       // 0: r[1] = 1
		v.AddOp(OpIf, 1, 4, 0)            // 1: if r[1] goto 4
		v.AddOp(OpInteger, 99, 2, 0)      // 2: r[2] = 99 (should skip)
		v.AddOp(OpInteger, -1, 2, 0)      // 3: r[2] = -1 (should skip)
		v.AddOp(OpInteger, 42, 2, 0)      // 4: r[2] = 42
		v.AddOp(OpHalt, 0, 0, 0)          // 5: Halt

		if err := v.Run(); err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		mem, _ := v.GetMem(2)
		if mem.IntValue() != 42 {
			t.Errorf("Expected 42 (jump taken), got %d", mem.IntValue())
		}
	})

	t.Run("Loop", func(t *testing.T) {
		v := New()
		v.AllocMemory(10)

		// Program: r[1] = 0; r[2] = 10; r[1]++; if r[1] < r[2] goto 2; Halt
		v.AddOp(OpInteger, 0, 1, 0)       // 0: r[1] = 0 (counter)
		v.AddOp(OpInteger, 10, 2, 0)      // 1: r[2] = 10 (limit)
		v.AddOp(OpInteger, 1, 3, 0)       // 2: r[3] = 1
		v.AddOp(OpAdd, 1, 3, 1)           // 3: r[1] = r[1] + 1
		v.AddOp(OpLt, 1, 2, 2)            // 4: if r[1] < r[2] goto 2
		v.AddOp(OpHalt, 0, 0, 0)          // 5: Halt

		if err := v.Run(); err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		mem, _ := v.GetMem(1)
		if mem.IntValue() != 10 {
			t.Errorf("Expected 10, got %d", mem.IntValue())
		}
	})
}

func TestVdbeComparison(t *testing.T) {
	tests := []struct {
		name   string
		opcode Opcode
		a, b   int64
		jump   bool
	}{
		{"EqTrue", OpEq, 5, 5, true},
		{"EqFalse", OpEq, 5, 10, false},
		{"NeTrue", OpNe, 5, 10, true},
		{"NeFalse", OpNe, 5, 5, false},
		{"LtTrue", OpLt, 5, 10, true},
		{"LtFalse", OpLt, 10, 5, false},
		{"LeTrue", OpLe, 5, 10, true},
		{"LeEqual", OpLe, 5, 5, true},
		{"LeFalse", OpLe, 10, 5, false},
		{"GtTrue", OpGt, 10, 5, true},
		{"GtFalse", OpGt, 5, 10, false},
		{"GeTrue", OpGe, 10, 5, true},
		{"GeEqual", OpGe, 5, 5, true},
		{"GeFalse", OpGe, 5, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			v.AllocMemory(10)

			// Program: r[1] = a; r[3] = b; r[4] = 0; compare and jump; r[4] = 1; Halt
			v.AddOp(OpInteger, int(tt.a), 1, 0)
			v.AddOp(OpInteger, int(tt.b), 3, 0)
			v.AddOp(OpInteger, 0, 4, 0)
			v.AddOp(tt.opcode, 1, 6, 3) // Jump to instruction 6 if condition true
			v.AddOp(OpInteger, 1, 4, 0)  // r[4] = 1 (not jumped)
			v.AddOp(OpHalt, 0, 0, 0)
			v.AddOp(OpInteger, 2, 4, 0)  // r[4] = 2 (jumped)
			v.AddOp(OpHalt, 0, 0, 0)

			if err := v.Run(); err != nil {
				t.Fatalf("Execution failed: %v", err)
			}

			mem, _ := v.GetMem(4)
			jumped := mem.IntValue() == 2

			if jumped != tt.jump {
				t.Errorf("Expected jump=%v, got jump=%v", tt.jump, jumped)
			}
		})
	}
}

func TestVdbeExplain(t *testing.T) {
	v := New()
	v.AllocMemory(10)

	v.AddOp(OpInteger, 42, 1, 0)
	v.SetComment(0, "Load constant 42")
	v.AddOp(OpResultRow, 1, 1, 0)
	v.SetComment(1, "Output result")
	v.AddOp(OpHalt, 0, 0, 0)

	explain := v.ExplainProgram()
	if explain == "" {
		t.Error("ExplainProgram should return non-empty string")
	}

	// Check that it contains expected opcode names
	if !contains(explain, "Integer") {
		t.Error("ExplainProgram should contain 'Integer' opcode")
	}
	if !contains(explain, "ResultRow") {
		t.Error("ExplainProgram should contain 'ResultRow' opcode")
	}
	if !contains(explain, "Halt") {
		t.Error("ExplainProgram should contain 'Halt' opcode")
	}
}

func TestVdbeCursorOperations(t *testing.T) {
	t.Run("OpenAndClose", func(t *testing.T) {
		v := New()
		v.AllocCursors(5)

		err := v.OpenCursor(0, CursorBTree, 1, true)
		if err != nil {
			t.Fatalf("OpenCursor failed: %v", err)
		}

		cursor, err := v.GetCursor(0)
		if err != nil {
			t.Fatalf("GetCursor failed: %v", err)
		}

		if cursor.CurType != CursorBTree {
			t.Error("Cursor type mismatch")
		}

		err = v.CloseCursor(0)
		if err != nil {
			t.Fatalf("CloseCursor failed: %v", err)
		}

		_, err = v.GetCursor(0)
		if err == nil {
			t.Error("Expected error when getting closed cursor")
		}
	})
}

func TestVdbeReset(t *testing.T) {
	v := New()
	v.AllocMemory(10)

	v.AddOp(OpInteger, 42, 1, 0)
	v.AddOp(OpHalt, 0, 0, 0)

	// Run once
	if err := v.Run(); err != nil {
		t.Fatalf("First run failed: %v", err)
	}

	mem, _ := v.GetMem(1)
	if mem.IntValue() != 42 {
		t.Error("First run didn't set value")
	}

	// Reset
	if err := v.Reset(); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	// Run again
	if err := v.Run(); err != nil {
		t.Fatalf("Second run failed: %v", err)
	}

	mem, _ = v.GetMem(1)
	if mem.IntValue() != 42 {
		t.Error("Second run didn't set value")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || contains(s[1:], substr)))
}
