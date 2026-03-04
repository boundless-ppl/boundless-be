package model

type UniversityType string

const (
	NATIONAL UniversityType = "NATIONAL"
	PUBLIC   UniversityType = "PUBLIC"
	PRIVATE  UniversityType = "PRIVATE"
)

type University struct {
	ID        string
	NegaraID  string
	Nama      string
	Kota      string
	Tipe      UniversityType
	Deskripsi string
	Website   string
	Ranking   *int
}
