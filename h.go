package sylph

type H map[string]interface{}

func (h H) Merge(mapping H) {
	if mapping == nil {
		return
	}

	for k, v := range mapping {
		h[k] = v
	}

	return
}
