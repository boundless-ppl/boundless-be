package model

import "time"

type User struct {
	UserID           string
	NamaLengkap      string
	Role             string
	Email            string
	PasswordHash     string
	CreatedAt        time.Time
	FailedLoginCount int
	FirstFailedAt    time.Time
	LockedUntil      time.Time
}
