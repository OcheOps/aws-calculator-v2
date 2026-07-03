package catalog

type Provider struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type Service struct {
	ID          int64  `json:"id"`
	ProviderID  int64  `json:"provider_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Family      string `json:"family,omitempty"`
	Description string `json:"description,omitempty"`
}

type Dimension struct {
	ID         int64  `json:"id"`
	Key        string `json:"key"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	ValueType  string `json:"value_type"`
	IsRequired bool   `json:"is_required"`
	SortOrder  int    `json:"sort_order"`
}

type Region struct {
	Code string `json:"code"`
}

type DimensionValue struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}
