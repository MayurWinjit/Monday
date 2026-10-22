package agent

// Merge starts from global, then overlays every project entry whose Name
// matches (replacing it), and appends project-only entries. Project always
// wins on a name clash. Pure function, no IO.
func Merge(global, project []Agent) []Agent {
	result := make([]Agent, 0, len(global)+len(project))
	index := make(map[string]int, len(global))

	for _, a := range global {
		index[a.Name] = len(result)
		result = append(result, a)
	}

	for _, a := range project {
		if i, ok := index[a.Name]; ok {
			result[i] = a
			continue
		}
		index[a.Name] = len(result)
		result = append(result, a)
	}

	return result
}
