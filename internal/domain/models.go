package domain

import "time"

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Token       string `json:"token,omitempty"`
}

type Chat struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	AvatarInitials  string    `json:"avatarInitials"`
	LastMessage     string    `json:"lastMessage"`
	LastMessageTime time.Time `json:"lastMessageTime"`
	UnreadCount     int32     `json:"unreadCount"`
	IsOnline        bool      `json:"isOnline"`
}

type Message struct {
	ID        string     `json:"id"`
	SenderID  string     `json:"senderId"`
	Text      string     `json:"text"`
	Timestamp time.Time  `json:"timestamp"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

type StoredChat struct {
	ID            string
	Participants  []string
	LastMessageID int64
	LastMessage   string
	LastMessageAt time.Time
}
