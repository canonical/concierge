package config

// MergeMaps takes two maps and returns a combined map, where KV pairs in the second map arg
// take precedence over the first.
func MergeMaps[K string, V any](m1 map[K]V, m2 map[K]V) map[K]V {
	combinedMap := map[K]V{}
	for k := range m1 {
		combinedMap[k] = m1[k]
	}
	for k := range m2 {
		combinedMap[k] = m2[k]
	}
	return combinedMap
}
