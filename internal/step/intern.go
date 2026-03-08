package step

// stringInterner deduplicates strings to reduce memory allocations.
// In IFC files there are only ~800 distinct type names, so interning
// saves significant allocations when parsing many entities.
type stringInterner struct {
	m map[string]string
}

func newStringInterner() *stringInterner {
	return &stringInterner{m: make(map[string]string, 1024)}
}

func (si *stringInterner) intern(s string) string {
	if existing, ok := si.m[s]; ok {
		return existing
	}
	si.m[s] = s
	return s
}
