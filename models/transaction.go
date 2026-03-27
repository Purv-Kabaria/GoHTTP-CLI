package models

import "time"

type HTTPTransaction struct {
	ID            string
	Method        string
	Host          string
	Path          string
	StatusCode    int
	ContentLength int64
	RequestTime   time.Time
	Duration      time.Duration
	SourceIP      string
	SourcePort    string
	DestIP        string
	DestPort      string
}