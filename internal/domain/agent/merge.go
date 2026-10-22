package agent

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
