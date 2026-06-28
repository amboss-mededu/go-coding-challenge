package model

import (
	"encoding/json"
	"fmt"
)

type Article struct {
	Authors    []string          `json:"authors,omitempty"`
	Category   Category          `json:"category,omitempty"`
	Content    *ArticleContent   `json:"content,omitempty"`
	Dimensions ArticleDimensions `json:"dimensions,omitempty"`
	Id         string            `json:"id"`
	Metadata   *ArticleMetadata  `json:"metadata,omitempty"`
	Status     ArticleStatus     `json:"status,omitempty"`
	Title      string            `json:"title"`
}

type ArticleContent struct {
	RichText  *ArticleRichText
	PlainText *ArticlePlainText
}

func (c *ArticleContent) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var disc struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(data, &disc); err != nil {
		return err
	}
	switch disc.Format {
	case "richtext":
		c.RichText = &ArticleRichText{}
		return json.Unmarshal(data, c.RichText)
	case "plaintext":
		c.PlainText = &ArticlePlainText{}
		return json.Unmarshal(data, c.PlainText)
	default:
		return fmt.Errorf("unknown format %q", disc.Format)
	}
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

type ArticlePlainText struct {
	Format string `json:"format,omitempty"`
	Text   string `json:"text,omitempty"`
}

type ArticleRichText struct {
	Format string                `json:"format,omitempty"`
	Html   string                `json:"html,omitempty"`
	Nodes  []ArticleRichTextNode `json:"nodes,omitempty"`
}

type ArticleRichTextNode struct {
	Children []ArticleRichTextNode `json:"children,omitempty"`
	Text     *string               `json:"text,omitempty"`
	Type     string                `json:"type,omitempty"`
}
