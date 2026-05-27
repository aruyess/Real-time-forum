package models

import "time"

type Comment struct {
	ID         string    `json:"id"`
	PostID     string    `json:"postId"`
	UserID     string    `json:"userId"`
	Author     string    `json:"author"` // nickname, joined from users
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
	Likes      int       `json:"likes"`
	Dislikes   int       `json:"dislikes"`
	MyReaction int       `json:"myReaction"`
}
