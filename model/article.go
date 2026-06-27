package model

type Article struct {
	Authors    []string          `json:"authors,omitempty"`
	Category   Category          `json:"category,omitempty"`
	Dimensions ArticleDimensions `json:"dimensions,omitempty"`
	Id         string            `json:"id"`
	Metadata   *ArticleMetadata  `json:"metadata,omitempty"`
	Status     ArticleStatus     `json:"status,omitempty"`
	Title      string            `json:"title"`
}

type ArticleDimensions struct {
	Height int `json:"height,omitempty"`
	Width  int `json:"width,omitempty"`
}

type ArticleMetadata struct {
	ReadingTime float64 `json:"readingTime,omitempty"`
	WordCount   int     `json:"wordCount,omitempty"`
}

type ArticleStatus string

const (
	ArticleStatusDraft     ArticleStatus = "draft"
	ArticleStatusPublished ArticleStatus = "published"
	ArticleStatusArchived  ArticleStatus = "archived"
)
