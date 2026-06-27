package model

type Article struct {
	Authors []string      `json:"authors,omitempty"`
	Id      string        `json:"id"`
	Status  ArticleStatus `json:"status,omitempty"`
	Title   string        `json:"title"`
}

type ArticleStatus string

const (
	ArticleStatusDraft     ArticleStatus = "draft"
	ArticleStatusPublished ArticleStatus = "published"
	ArticleStatusArchived  ArticleStatus = "archived"
)
