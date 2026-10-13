package telegram

import (
	"context"
	"time"
)

type HandlerFunc func(ctx context.Context, args string) string

type Conversation interface {
	Answer(ctx context.Context, text string) (reply string, done bool)
}

type StartFunc func(ctx context.Context, args string) (prompt string, conv Conversation)

type command struct {
	handler     HandlerFunc
	start       StartFunc
	description string
}

type session struct {
	command string
	conv    Conversation
	expires time.Time
}

type apiResponse struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
	Date      int    `json:"date"`
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}
