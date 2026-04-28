package model

import "time"

type DocumentType string

const (
	DocumentTypeTranscript   DocumentType = "transcript"
	DocumentTypeCV           DocumentType = "cv"
	DocumentTypePaymentProof DocumentType = "payment_proof"
)

type Document struct {
	DocumentID       string
	UserID           string
	Nama             string
	OriginalFilename string
	StoragePath      string
	DokumenURL       string
	PublicURL        string
	MIMEType         string
	SizeBytes        int64
	DokumenSizeKB    int64
	DocumentType     DocumentType
	Source           string
	UploadedAt       time.Time
}
