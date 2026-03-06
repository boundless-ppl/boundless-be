package dto

type CreateUniversityRequest struct {
	NegaraID  string `json:"negara_id" binding:"required"`
	Nama      string `json:"nama" binding:"required"`
	Kota      string `json:"kota" binding:"required"`
	Tipe      string `json:"tipe" binding:"required"`
	Deskripsi string `json:"deskripsi"`
	Website   string `json:"website"`
	Ranking   *int   `json:"ranking"`
}

type UpdateUniversityRequest struct {
	NegaraID  string `json:"negara_id"`
	Nama      string `json:"nama"`
	Kota      string `json:"kota"`
	Tipe      string `json:"tipe"`
	Deskripsi string `json:"deskripsi"`
	Website   string `json:"website"`
	Ranking   *int   `json:"ranking"`
}

type UniversityResponse struct {
	ID        string `json:"id"`
	NegaraID  string `json:"negara_id"`
	Nama      string `json:"nama"`
	Kota      string `json:"kota"`
	Tipe      string `json:"tipe"`
	Deskripsi string `json:"deskripsi"`
	Website   string `json:"website"`
	Ranking   *int   `json:"ranking,omitempty"`
}
