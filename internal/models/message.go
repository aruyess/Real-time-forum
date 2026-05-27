package models

import "time"

type Message struct {
	ID             string    `json:"id"`
	SenderID       string    `json:"senderId"`
	ReceiverID     string    `json:"receiverId"`
	SenderNickname string    `json:"senderNickname"`
	Content        string    `json:"content"`
	ImageURL       string    `json:"imageUrl,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}
