package dto

type SubscriptionPackageResponse struct {
	SubscriptionID string   `json:"subscription_id"`
	PackageKey     string   `json:"package_key"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	DurationMonths int      `json:"duration_months"`
	PriceAmount    int64    `json:"price_amount"`
	Benefits       []string `json:"benefits"`
}

type ListSubscriptionPackagesResponse struct {
	Packages []SubscriptionPackageResponse `json:"packages"`
}

type CreatePaymentRequest struct {
	SubscriptionID string `json:"subscription_id" binding:"required"`
}

type CreatePaymentResponse struct {
	PaymentID     string `json:"payment_id"`
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`

	PackageName    string   `json:"package_name"`
	DurationMonths int      `json:"duration_months"`
	TotalAmount    int64    `json:"total_amount"`
	Benefits       []string `json:"benefits"`

	QrisImageURL string `json:"qris_image_url"`
	CreatedAt    string `json:"created_at"`
	ExpiredAt    string `json:"expired_at"`
}

type PaymentDetailResponse struct {
	PaymentID     string `json:"payment_id"`
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`

	PackageName    string   `json:"package_name"`
	DurationMonths int      `json:"duration_months"`
	TotalAmount    int64    `json:"total_amount"`
	Benefits       []string `json:"benefits"`

	QrisImageURL    string  `json:"qris_image_url"`
	ProofDocumentID *string `json:"proof_document_id,omitempty"`

	CreatedAt string  `json:"created_at"`
	ExpiredAt string  `json:"expired_at"`
	PaidAt    *string `json:"paid_at,omitempty"`

	PremiumActiveAt  *string `json:"premium_active_at,omitempty"`
	PremiumExpiredAt *string `json:"premium_expired_at,omitempty"`
}

type AdminPaymentListItemResponse struct {
	PaymentID     string `json:"payment_id"`
	TransactionID string `json:"transaction_id"`
	UserID        string `json:"user_id"`
	UserName      string `json:"user_name"`

	PackageName string `json:"package_name"`
	Amount      int64  `json:"amount"`

	Status           string  `json:"status"`
	TransactionDate  string  `json:"transaction_date"`
	ProofDocumentID  *string `json:"proof_document_id,omitempty"`
	ProofDocumentURL *string `json:"proof_document_url,omitempty"`
}

type AdminUpdatePaymentStatusRequest struct {
	Status          string  `json:"status" binding:"required"`
	StartDate       *string `json:"start_date,omitempty"`
	AdminNote       *string `json:"admin_note,omitempty"`
	ProofDocumentID *string `json:"proof_document_id,omitempty"`
}

type AdminPaymentListResponse struct {
	Items []AdminPaymentListItemResponse `json:"payments"`
}

type AdminUpdatePaymentStatusResponse struct {
	PaymentID        string  `json:"payment_id"`
	TransactionID    string  `json:"transaction_id"`
	Status           string  `json:"status"`
	PaidAt           *string `json:"paid_at,omitempty"`
	PremiumActiveAt  *string `json:"premium_active_at,omitempty"`
	PremiumExpiredAt *string `json:"premium_expired_at,omitempty"`
}

type UploadPaymentProofResponse struct {
	DocumentID       string `json:"document_id"`
	OriginalFilename string `json:"original_filename"`
	PublicURL        string `json:"public_url"`
	MIMEType         string `json:"mime_type"`
	SizeBytes        int64  `json:"size_bytes"`
	DocumentType     string `json:"document_type"`
	UploadedAt       string `json:"uploaded_at"`
}
