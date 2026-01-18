package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
}

type Chat struct {
	ID         uuid.UUID
	Name       string
	OwnerID    uuid.UUID
	InviteCode string
	CreatedAt  time.Time
}

type Message struct {
	ID        uuid.UUID
	ChatID    uuid.UUID
	UserID    uuid.UUID
	Content   string
	CreatedAt time.Time
}
