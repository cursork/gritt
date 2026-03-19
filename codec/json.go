package codec

import "encoding/json"

// ToJSON converts APLAN-parsed values to JSON-safe structures.
//
//   - *Array becomes {"type":"array","shape":[...],"data":[...]}
//   - *Namespace becomes {"type":"namespace","data":{...}}
//   - complex128 becomes {"re":...,"im":...}
//   - *zilde (Zilde) becomes []
//   - []any elements are recursively converted
//   - int, float64, string, nil pass through
func ToJSON(v any) any {
	switch val := v.(type) {
	case *Array:
		return map[string]any{
			"type":  "array",
			"shape": val.Shape,
			"data":  sliceToJSON(val.Data),
		}
	case *Namespace:
		m := make(map[string]any, len(val.Keys))
		for _, k := range val.Keys {
			m[k] = ToJSON(val.Values[k])
		}
		return map[string]any{
			"type": "namespace",
			"data": m,
		}
	case []any:
		result := make([]any, len(val))
		for i, el := range val {
			result[i] = ToJSON(el)
		}
		return result
	case complex128:
		return map[string]any{
			"re": real(val),
			"im": imag(val),
		}
	case *zilde:
		return []any{}
	default:
		return v
	}
}

// ToJSONBytes is a convenience that calls ToJSON and marshals to JSON bytes.
func ToJSONBytes(v any) ([]byte, error) {
	return json.Marshal(ToJSON(v))
}

// FromJSON converts a JSON-decoded value (from encoding/json) into
// codec types suitable for Serialize.
//
// JSON types map as follows:
//   - object with "type":"array" → *Array (round-trip from ToJSON)
//   - object with "type":"namespace" → *Namespace (round-trip from ToJSON)
//   - other objects → *Namespace (keys sorted by insertion order from JSON)
//   - arrays → []any
//   - float64 → int (if whole number) or float64
//   - string, bool, nil pass through
//
// When lossy is true, shaped arrays are NOT reconstructed — nested JSON
// arrays stay as nested []any, which won't round-trip shape metadata.
func FromJSON(v any, lossy bool) any {
	switch val := v.(type) {
	case map[string]any:
		// Check for round-trip typed structures
		if !lossy {
			if typ, ok := val["type"].(string); ok {
				switch typ {
				case "array":
					return arrayFromJSON(val)
				case "namespace":
					return namespaceFromJSON(val)
				}
			}
		}
		// Generic object → Namespace
		ns := &Namespace{Values: make(map[string]any, len(val))}
		for k, v := range val {
			ns.Keys = append(ns.Keys, k)
			ns.Values[k] = FromJSON(v, lossy)
		}
		return ns
	case []any:
		result := make([]any, len(val))
		for i, el := range val {
			result[i] = FromJSON(el, lossy)
		}
		return result
	case float64:
		// JSON numbers are always float64; convert whole numbers to int
		if val == float64(int64(val)) {
			return int(int64(val))
		}
		return val
	default:
		return v
	}
}

func arrayFromJSON(m map[string]any) any {
	shapeRaw, ok := m["shape"].([]any)
	if !ok {
		return m // can't reconstruct
	}
	shape := make([]int, len(shapeRaw))
	for i, s := range shapeRaw {
		if f, ok := s.(float64); ok {
			shape[i] = int(f)
		}
	}
	dataRaw, ok := m["data"].([]any)
	if !ok {
		return m
	}
	data := make([]any, len(dataRaw))
	for i, el := range dataRaw {
		data[i] = FromJSON(el, false)
	}
	return &Array{Shape: shape, Data: data}
}

func namespaceFromJSON(m map[string]any) any {
	dataRaw, ok := m["data"].(map[string]any)
	if !ok {
		return m
	}
	ns := &Namespace{Values: make(map[string]any, len(dataRaw))}
	for k, v := range dataRaw {
		ns.Keys = append(ns.Keys, k)
		ns.Values[k] = FromJSON(v, false)
	}
	return ns
}

func sliceToJSON(data []any) []any {
	result := make([]any, len(data))
	for i, el := range data {
		result[i] = ToJSON(el)
	}
	return result
}
