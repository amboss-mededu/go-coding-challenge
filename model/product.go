package model

type Product struct {
	Active         bool    `json:"active,omitempty"`
	Classification string  `json:"classification,omitempty"`
	Description    *string `json:"description,omitempty"`
	Id             string  `json:"id"`
	Name           string  `json:"name"`
	Price          float64 `json:"price"`
	Quantity       int     `json:"quantity,omitempty"`
}
