package codec

import "fmt"

// Get indexes into an APLAN value by one or more indices.
// For *Array, index rank must match the array's rank.
// For []any, indices traverse nested slices.
func Get(value any, indices ...int) (any, error) {
	if _, ok := value.(*zilde); ok {
		return nil, fmt.Errorf("cannot index into zilde (empty array)")
	}

	if len(indices) == 0 {
		return value, nil
	}

	switch v := value.(type) {
	case *Array:
		if len(indices) != len(v.Shape) {
			return nil, fmt.Errorf("index rank %d does not match array rank %d", len(indices), len(v.Shape))
		}
		var current any = v.Data
		for i, idx := range indices {
			if idx < 0 || idx >= v.Shape[i] {
				return nil, fmt.Errorf("index %d out of bounds for dimension %d with size %d", idx, i, v.Shape[i])
			}
			arr, ok := current.([]any)
			if !ok {
				return current, nil
			}
			current = arr[idx]
		}
		return current, nil

	case []any:
		var current any = v
		for i, idx := range indices {
			arr, ok := current.([]any)
			if !ok {
				return nil, fmt.Errorf("cannot index deeper: reached non-array at depth %d", i)
			}
			if idx < 0 || idx >= len(arr) {
				return nil, fmt.Errorf("index %d out of bounds for array of length %d", idx, len(arr))
			}
			current = arr[idx]
		}
		return current, nil
	}

	return nil, fmt.Errorf("cannot index into value of type %T", value)
}
