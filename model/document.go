package model

import "time"

type DocumentType string

const (
	DocumentTypeTranscript DocumentType = "transcript"
	DocumentTypeCV         DocumentType = "cv"
)

type Document struct {
	DocumentID       string
	UserID           string
	OriginalFilename string
	StoragePath      string
	PublicURL        string
	MIMEType         string
	SizeBytes        int64
	DocumentType     DocumentType
	Source           string
	UploadedAt       time.Time
}
