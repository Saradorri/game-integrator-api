package domain

import (
	"time"

	"gorm.io/gorm"
)

// User represents a player in the system
type User struct {
	ID        int            `json:"user_id" gorm:"primaryKey;column:id;type:integer"`
	Username  string         `json:"username" gorm:"uniqueIndex;not null;type:varchar(64)"`
	Password  string         `json:"-" gorm:"not null;type:varchar(128)"`
	Balance   float64        `json:"balance" gorm:"type:numeric(20,2);not null;default:0"`
	Currency  string         `json:"currency" gorm:"type:varchar(8);not null"`
	CreatedAt time.Time      `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"not null"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName specifies the table name for User
func (u User) TableName() string {
	return "users"
}

// UserRepository defines the interface for user data
type UserRepository interface {
	GetByID(id int) (*User, error)
	GetByUsername(username string) (*User, error)
	Create(user *User) error
	Update(user *User) error
	UpdateBalance(userID int, newBalance float64) error
}

// UserUseCase defines the interface for user business logic
type UserUseCase interface {
	Authenticate(username, password string) (string, error)
	GetUserInfo(userID int) (*User, error)
}
