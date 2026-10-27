package kv

import "fmt"

// ResolvePredicate binds a predicate to the source schema. A nil predicate
// means all tuples match and remains nil.
func ResolvePredicate(schema Schema, pred *Expr) (*Expr, error) {
	if pred == nil {
		return nil, nil
	}
	resolved, err := pred.Resolve(schema)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

// ResolveProjection returns source positions and an output schema in the
// requested column order. An empty project list selects all columns.
func ResolveProjection(schema Schema, project []string) ([]int, Schema, error) {
	out := Schema{Name: schema.Name}
	var positions []int
	if len(project) == 0 {
		for i, attr := range schema.Attributes {
			positions = append(positions, i)
			out.Attributes = append(out.Attributes, attr)
		}
		return positions, out, nil
	}
	for _, name := range project {
		pos, found := schema.AttributeIndex(name)
		if !found {
			return nil, out, fmt.Errorf("unknown projection column %q", name)
		}
		positions = append(positions, pos)
		out.Attributes = append(out.Attributes, schema.Attributes[pos])
	}
	return positions, out, nil
}

// MatchesPredicate evaluates a resolved predicate against a source tuple,
// requiring a boolean result. A nil predicate matches every tuple.
func MatchesPredicate(tuple Tuple, pred *Expr) (bool, error) {
	if pred == nil {
		return true, nil
	}
	res, err := pred.Eval(tuple)
	if err != nil {
		return false, err
	}
	if res.DataType != DataTypeBool {
		return false, fmt.Errorf("predicate must return bool, got %s", res.DataType)
	}
	return res.Bool, nil
}

// ProjectTuple copies values at positions returned by ResolveProjection.
// The tuple must follow the source schema used to resolve the projection.
func ProjectTuple(tuple Tuple, positions []int) Tuple {
	projected := Tuple{Values: make([]Value, len(positions))}
	for i, position := range positions {
		projected.Values[i] = tuple.Values[position]
	}
	return projected
}
