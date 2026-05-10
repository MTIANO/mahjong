package model

import "time"

type User struct {
	ID        int64     `json:"id"`
	OpenID    string    `json:"openid"`
	Nickname  string    `json:"nickname"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
