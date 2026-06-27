package model

type Product struct {
	Active         bool                  `json:"active,omitempty"`
	Classification ProductClassification `json:"classification,omitempty"`
	Description    *string               `json:"description,omitempty"`
	Dimensions     ProductDimensions     `json:"dimensions,omitempty"`
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

type ProductDimensionsUnit string

const (
	ProductDimensionsUnitCm   ProductDimensionsUnit = "cm"
	ProductDimensionsUnitInch ProductDimensionsUnit = "inch"
)

type ProductDimensions struct {
	Height float64               `json:"height,omitempty"`
	Unit   ProductDimensionsUnit `json:"unit,omitempty"`
	Width  float64               `json:"width,omitempty"`
}
