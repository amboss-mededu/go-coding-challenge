package model

type Category struct {
	Id     string  `json:"id,omitempty"`
	Kind   string  `json:"kind,omitempty"`
	Name   string  `json:"name,omitempty"`
	Parent *string `json:"parent,omitempty"`
}
