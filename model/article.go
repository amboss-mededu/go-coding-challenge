package model

type Article struct {
	Id     string `json:"id"`
	Status string `json:"status,omitempty"`
	Title  string `json:"title"`
}
