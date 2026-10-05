package kv

import "testing"

// Working on writing idiomatic Go tests. LLM guided me through table tests with TestGT.

func TestOr(t *testing.T) {
	tests := []struct {
		name  string
		left  bool
		right bool
		want  bool
	}{
		// Truth table
		{name: "bothTrue", left: true, right: true, want: true},
		{name: "firstTrue", left: true, right: false, want: true},
		{name: "secondTrue", left: false, right: true, want: true},
		{name: "bothFalse", left: false, right: false, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := Binary(OpOr, Literal(BoolValue(tt.left)), Literal(BoolValue(tt.right)))
			got, err := predicate.Eval(NullValue())
			if err != nil {
				t.Fatalf("Eval() error = %v", err)
			}
			if got.Kind != KindBool || got.Bool != tt.want {
				t.Errorf("Eval(%+v OR %+v) = %+v, want bool %t", tt.left, tt.right, got, tt.want)
			}
		})
	}
}

func TestLT(t *testing.T) {
	tests := []struct {
		name  string
		left  int64
		right int64
		want  bool
	}{
		{name: "greater", left: 5, right: 4, want: false},
		{name: "equal", left: 4, right: 4, want: false},
		{name: "less", left: 3, right: 4, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := Binary(OpLT, Literal(IntValue(tt.left)), Literal(IntValue(tt.right)))
			got, err := predicate.Eval(NullValue())
			if err != nil {
				t.Fatalf("Eval() error = %v", err)
			}
			if got.Kind != KindBool || got.Bool != tt.want {
				t.Errorf("Eval(%d < %d) = %+v, want bool %t", tt.left, tt.right, got, tt.want)
			}
		})
	}
}

func TestGT(t *testing.T) {
	tests := []struct {
		name  string
		left  int64
		right int64
		want  bool
	}{
		{name: "greater", left: 5, right: 4, want: true},
		{name: "equal", left: 4, right: 4, want: false},
		{name: "less", left: 3, right: 4, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := Binary(OpGT, Literal(IntValue(tt.left)), Literal(IntValue(tt.right)))
			got, err := predicate.Eval(NullValue())
			if err != nil {
				t.Fatalf("Eval() error = %v", err)
			}
			if got.Kind != KindBool || got.Bool != tt.want {
				t.Errorf("Eval(%d > %d) = %+v, want bool %t", tt.left, tt.right, got, tt.want)
			}
		})
	}
}

func TestAnd(t *testing.T) {
	tests := []struct {
		name  string
		left  bool
		right bool
		want  bool
	}{
		// Truth table
		{name: "bothTrue", left: true, right: true, want: true},
		{name: "firstTrue", left: true, right: false, want: false},
		{name: "secondTrue", left: false, right: true, want: false},
		{name: "bothFalse", left: false, right: false, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := Binary(OpAnd, Literal(BoolValue(tt.left)), Literal(BoolValue(tt.right)))
			got, err := predicate.Eval(NullValue())
			if err != nil {
				t.Fatalf("Eval() error = %v", err)
			}
			if got.Kind != KindBool || got.Bool != tt.want {
				t.Errorf("Eval(%+v AND %+v) = %+v, want bool %t", tt.left, tt.right, got, tt.want)
			}
		})
	}
}

func TestAndShortCircuit(t *testing.T) {
	predicate := Binary(OpAnd,
		Literal(BoolValue(false)),
		Binary(OpGT, Literal(BoolValue(true)), Literal(BoolValue(false))),
	)
	got, err := predicate.Eval(NullValue())
	if err != nil {
		t.Fatalf("Eval() error = %v; expected the right operand to be skipped", err)
	}
	if got.Kind != KindBool || got.Bool {
		t.Errorf("Eval() = %+v, want bool false", got)
	}
}
