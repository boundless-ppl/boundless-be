package dto

type ScholarshipListQuery struct {
	Page           int    `form:"page"`
	PageSize       int    `form:"page_size"`
	Search         string `form:"search"`
	TipePembiayaan string `form:"tipe_pembiayaan"`
	Negara         string `form:"negara"`
}

type ScholarshipUniversityResponse struct {
	UniversityID string  `json:"university_id"`
	Nama         string  `json:"nama"`
	Kota         string  `json:"kota"`
	Negara       string  `json:"negara"`
	Ranking      *int    `json:"ranking,omitempty"`
	Website      *string `json:"website,omitempty"`
	Tipe         *string `json:"tipe,omitempty"`
	Deskripsi    *string `json:"deskripsi,omitempty"`
}

type ScholarshipResponse struct {
	ID              string                          `json:"id"`
	Nama            string                          `json:"nama"`
	Provider        string                          `json:"provider"`
	Deskripsi       string                          `json:"deskripsi"`
	Persyaratan     []string                        `json:"persyaratan"`
	Benefit         []string                        `json:"benefit"`
	Deadline        string                          `json:"deadline"`
	LinkPendaftaran string                          `json:"link_pendaftaran"`
	TipePembiayaan  string                          `json:"tipe_pembiayaan,omitempty"`
	Negara          string                          `json:"negara,omitempty"`
	IsActive        bool                            `json:"is_active"`
	Universitas     []ScholarshipUniversityResponse `json:"universitas,omitempty"`
}

type ScholarshipListResponse struct {
	Data      []ScholarshipResponse `json:"data"`
	Total     int                   `json:"total"`
	Page      int                   `json:"page"`
	PageSize  int                   `json:"page_size"`
	TotalPage int                   `json:"total_pages"`
}
