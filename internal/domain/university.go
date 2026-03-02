package domain

type UniversityType string

const (
	NATIONAL UniversityType = "NATIONAL"
	PUBLIC   UniversityType = "PUBLIC"
	PRIVATE  UniversityType = "PRIVATE"
)

type University struct {
	ID        string         `json:"id"`
	NegaraID  string         `json:"negara_id"`
	Nama      string         `json:"nama"`
	Kota      string         `json:"kota"`
	Tipe      UniversityType `json:"tipe"`
	Deskripsi string         `json:"deskripsi"`
	Website   string         `json:"website"`
	Ranking   *int           `json:"ranking,omitempty"`
}
