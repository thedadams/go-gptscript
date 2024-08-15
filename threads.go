package gptscript

import "time"

type Thread struct {
	ID         uint64    `json:"id"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	FirstRunID uint64    `json:"firstRunID"`
}
