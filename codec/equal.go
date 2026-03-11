package codec

import "math"

// Equal reports whether two APLAN values are semantically equal.
// Handles Zilde, complex128, *Array (shape+data), *Namespace, []any, and scalars.
func Equal(a, b any) bool {
	// Zilde ↔ empty slice
	aZ := isZilde(a)
	bZ := isZilde(b)
	if aZ && isEmptySlice(b) || bZ && isEmptySlice(a) {
		return true
	}
	if aZ && bZ {
		return true
	}

	switch av := a.(type) {
	case int:
		bv, ok := b.(int)
		return ok && av == bv
	case float64:
		bv, ok := b.(float64)
		if !ok {
			return false
		}
		if math.IsNaN(av) && math.IsNaN(bv) {
			return true
		}
		return av == bv
	case complex128:
		bv, ok := b.(complex128)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case []any:
		bv, ok := b.([]any)
		if !ok {
			return false
		}
		if len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !Equal(av[i], bv[i]) {
				return false
			}
		}
		return true
	case *Array:
		bv, ok := b.(*Array)
		if !ok {
			return false
		}
		if len(av.Shape) != len(bv.Shape) {
			return false
		}
		for i := range av.Shape {
			if av.Shape[i] != bv.Shape[i] {
				return false
			}
		}
		if len(av.Data) != len(bv.Data) {
			return false
		}
		for i := range av.Data {
			if !Equal(av.Data[i], bv.Data[i]) {
				return false
			}
		}
		return true
	case *Namespace:
		bv, ok := b.(*Namespace)
		if !ok {
			return false
		}
		if len(av.Keys) != len(bv.Keys) {
			return false
		}
		for i, k := range av.Keys {
			if k != bv.Keys[i] {
				return false
			}
			if !Equal(av.Values[k], bv.Values[k]) {
				return false
			}
		}
		return true
	}

	return false
}

func isZilde(v any) bool {
	_, ok := v.(*zilde)
	return ok
}

func isEmptySlice(v any) bool {
	s, ok := v.([]any)
	return ok && len(s) == 0
}
