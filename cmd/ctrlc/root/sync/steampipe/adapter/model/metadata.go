package model

type Metadata map[string]string

type Tags = map[string]string

func (m Metadata) AppendTags(tags Tags) Metadata {
	for k, v := range tags {
		m[k] = v
	}
	return m
}
