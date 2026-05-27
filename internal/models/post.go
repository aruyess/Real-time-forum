package models

import "time"

type Post struct {
	ID         string    `json:"id"`
	UserID     string    `json:"userId"`
	Author     string    `json:"author"` // nickname, joined from users
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Categories []string  `json:"categories"`
	CreatedAt  time.Time `json:"createdAt"`
	Likes      int       `json:"likes"`
	Dislikes   int       `json:"dislikes"`
	MyReaction int       `json:"myReaction"` // -1, 0 or 1 — 0 means anonymous or no reaction
}

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
