package models

import (
	"net/http"
	"time"
)

type HTTPTransaction struct {
	ID            string
	Protocol      string
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
	ReqHeaders    http.Header
	ResHeaders    http.Header
	ReqBody       []byte
	ResBody       []byte
}