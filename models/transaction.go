package models

import "time"

type HTTPTransaction struct {
	ID         string
	Method     string
	Host       string
	Path       string
	StatusCode int
	Duration   time.Duration
	SourceIP   string
	DestIP     string
}