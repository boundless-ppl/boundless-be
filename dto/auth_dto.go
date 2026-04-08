package dto

type RegisterRequest struct {
	NamaLengkap string `json:"nama_lengkap" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type MeResponse struct {
	UserID         string  `json:"user_id"`
	NamaLengkap    string  `json:"nama_lengkap"`
	Email          string  `json:"email"`
	Role           string  `json:"role"`
	IsPremium      bool    `json:"is_premium"`
	PremiumStartAt *string `json:"premium_start_at,omitempty"`
	PremiumEndAt   *string `json:"premium_end_at,omitempty"`
}
