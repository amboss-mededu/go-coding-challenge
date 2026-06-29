package model

import (
	"encoding/json"
	"fmt"
)

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

func (e *ProductClassification) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch ProductClassification(s) {
	case ProductClassificationBasicScience, ProductClassificationBasicScience2, ProductClassificationApplied, ProductClassificationExperimental:
		*e = ProductClassification(s)
		return nil
	default:
		return fmt.Errorf("invalid ProductClassification %q", s)
	}
}

type ProductDimensionsUnit string

const (
	ProductDimensionsUnitCm   ProductDimensionsUnit = "cm"
	ProductDimensionsUnitInch ProductDimensionsUnit = "inch"
)

func (e *ProductDimensionsUnit) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch ProductDimensionsUnit(s) {
	case ProductDimensionsUnitCm, ProductDimensionsUnitInch:
		*e = ProductDimensionsUnit(s)
		return nil
	default:
		return fmt.Errorf("invalid ProductDimensionsUnit %q", s)
	}
}

type ProductDimensions struct {
	Height float64               `json:"height,omitempty"`
	Unit   ProductDimensionsUnit `json:"unit,omitempty"`
	Width  float64               `json:"width,omitempty"`
}
