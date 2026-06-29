package model

import (
	"encoding/json"
	"fmt"
)

type Category struct {
	Id     string       `json:"id,omitempty"`
	Kind   CategoryKind `json:"kind,omitempty"`
	Name   string       `json:"name,omitempty"`
	Parent *string      `json:"parent,omitempty"`
}

type CategoryKind string

const (
	CategoryKindBasicScience  CategoryKind = "basic-science"
	CategoryKindBasicScience2 CategoryKind = "basic_science"
	CategoryKindClinical      CategoryKind = "clinical"
	CategoryKindOther         CategoryKind = "other"
)

func (e *CategoryKind) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch CategoryKind(s) {
	case CategoryKindBasicScience, CategoryKindBasicScience2, CategoryKindClinical, CategoryKindOther:
		*e = CategoryKind(s)
		return nil
	default:
		return fmt.Errorf("invalid CategoryKind %q", s)
	}
}
