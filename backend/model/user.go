package model

import "time"

// Role constants for dashboard users.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// User represents a dashboard login account.
type User struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	Username       string    `gorm:"uniqueIndex;not null" json:"username"`
	Email          string    `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash   string    `gorm:"not null" json:"-"`
	Role           string    `gorm:"default:user" json:"role"`
	XUIClientEmail string    `json:"xuiClientEmail"`
	XUISubID       string    `json:"xuiSubId"`
	Balance        float64   `gorm:"default:0" json:"balance"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
