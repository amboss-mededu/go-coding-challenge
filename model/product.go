package model

type Product struct {
	Active         bool                  `json:"active,omitempty"`
	Classification ProductClassification `json:"classification,omitempty"`
	Description    *string               `json:"description,omitempty"`
	Id             string                `json:"id"`
	Name           string                `json:"name"`
	Price          float64               `json:"price"`
	Quantity       int                   `json:"quantity,omitempty"`
	Tags           []string              `json:"tags,omitempty"`
}

type ProductClassification string

const (
	ProductClassificationBasicScience  ProductClassification = "basic_science"
	ProductClassificationBasicScience2 ProductClassification = "basic-science"
	ProductClassificationApplied       ProductClassification = "applied"
	ProductClassificationExperimental  ProductClassification = "experimental"
)
