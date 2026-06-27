package model

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
