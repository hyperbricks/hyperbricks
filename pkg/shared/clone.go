package shared

// CloneMapDeep performs a deep copy of map[string]interface{} values,
// recursively copying nested maps and slices.
func CloneMapDeep(source map[string]interface{}) map[string]interface{} {
	if source == nil {
		return nil
	}
	dest := make(map[string]interface{}, len(source))
	for k, v := range source {
		dest[k] = cloneValueDeep(v)
	}
	return dest
}

func cloneValueDeep(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return CloneMapDeep(typed)
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, elem := range typed {
			out[i] = cloneValueDeep(elem)
		}
		return out
	default:
		return value
	}
}
