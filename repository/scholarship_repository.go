package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"boundless-be/errs"
	"boundless-be/model"
)

type ScholarshipListFilter struct {
	Page           int
	PageSize       int
	Search         string
	TipePembiayaan string
	Negara         string
}

type ScholarshipItem struct {
	Funding       model.FundingOption
	Deadline      string
	Negara        string
	Persyaratan   []string
	Benefit       []string
	Universitas   []model.University
	LinkDaftarURL string
	IsActive      bool
}

type ScholarshipListResult struct {
	Data      []ScholarshipItem
	Total     int
	Page      int
	PageSize  int
	TotalPage int
}

type ScholarshipRepository interface {
	List(ctx context.Context, filter ScholarshipListFilter) (ScholarshipListResult, error)
	FindByID(ctx context.Context, id string) (ScholarshipItem, error)
}

type DBScholarshipRepository struct {
	db *sql.DB
}

func NewScholarshipRepository(db *sql.DB) *DBScholarshipRepository {
	return &DBScholarshipRepository{db: db}
}

func (r *DBScholarshipRepository) List(ctx context.Context, filter ScholarshipListFilter) (ScholarshipListResult, error) {
	countryExpr := countryLabelSQL("u.negara_id")
	whereParts := []string{"1=1"}
	args := make([]any, 0, 8)
	argPos := 1

	if s := strings.TrimSpace(filter.Search); s != "" {
		whereParts = append(whereParts, fmt.Sprintf("(LOWER(fo.nama_beasiswa) LIKE LOWER($%d) OR LOWER(fo.provider) LIKE LOWER($%d))", argPos, argPos))
		args = append(args, "%"+s+"%")
		argPos++
	}

	if s := strings.TrimSpace(filter.TipePembiayaan); s != "" {
		normalized := strings.ToLower(s)
		switch normalized {
		case "penuh":
			whereParts = append(whereParts, "fo.tipe_pembiayaan IN ('SCHOLARSHIP','ASSISTANTSHIP','SPONSORSHIP')")
		case "parsial":
			whereParts = append(whereParts, "fo.tipe_pembiayaan IN ('SELF_FUNDED','LOAN')")
		default:
			whereParts = append(whereParts, fmt.Sprintf("fo.tipe_pembiayaan = $%d", argPos))
			args = append(args, s)
			argPos++
		}
	}

	if s := strings.TrimSpace(filter.Negara); s != "" {
		whereParts = append(whereParts, fmt.Sprintf("(LOWER(%s) = LOWER($%d) OR LOWER(u.negara_id) = LOWER($%d))", countryExpr, argPos, argPos))
		args = append(args, s)
		argPos++
	}

	whereClause := strings.Join(whereParts, " AND ")

	countQuery := `
		SELECT COUNT(DISTINCT fo.funding_id)
		FROM funding_options fo
		LEFT JOIN admission_funding af ON af.funding_id = fo.funding_id
		LEFT JOIN admission_paths ap ON ap.admission_id = af.admission_id
		LEFT JOIN programs p ON p.program_id = ap.program_id
		LEFT JOIN universities u ON u.id = p.university_id
		WHERE ` + whereClause
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return ScholarshipListResult{}, fmt.Errorf("count scholarships: %w", err)
	}

	offset := (filter.Page - 1) * filter.PageSize
	listQuery := `
		SELECT
			fo.funding_id,
			fo.nama_beasiswa,
			fo.deskripsi,
			fo.provider,
			fo.tipe_pembiayaan,
			fo.website,
			COALESCE(TO_CHAR(MIN(ap.deadline), 'YYYY-MM-DD'), ''),
			COALESCE(MIN(` + countryExpr + `), '')
		FROM funding_options fo
		LEFT JOIN admission_funding af ON af.funding_id = fo.funding_id
		LEFT JOIN admission_paths ap ON ap.admission_id = af.admission_id
		LEFT JOIN programs p ON p.program_id = ap.program_id
		LEFT JOIN universities u ON u.id = p.university_id
		WHERE ` + whereClause + `
		GROUP BY fo.funding_id, fo.nama_beasiswa, fo.deskripsi, fo.provider, fo.tipe_pembiayaan, fo.website
		ORDER BY MIN(ap.deadline) ASC NULLS LAST, fo.nama_beasiswa ASC
		LIMIT $` + fmt.Sprintf("%d", argPos) + ` OFFSET $` + fmt.Sprintf("%d", argPos+1)
	listArgs := append(args, filter.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return ScholarshipListResult{}, fmt.Errorf("list scholarships: %w", err)
	}
	defer rows.Close()

	items := make([]ScholarshipItem, 0)
	for rows.Next() {
		var item ScholarshipItem
		var deskripsi sql.NullString
		if err := rows.Scan(
			&item.Funding.FundingID,
			&item.Funding.NamaBeasiswa,
			&deskripsi,
			&item.Funding.Provider,
			&item.Funding.TipePembiayaan,
			&item.Funding.Website,
			&item.Deadline,
			&item.Negara,
		); err != nil {
			return ScholarshipListResult{}, fmt.Errorf("scan scholarships: %w", err)
		}
		if deskripsi.Valid {
			item.Funding.Deskripsi = &deskripsi.String
		}
		item.LinkDaftarURL = item.Funding.Website
		item.IsActive = true
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ScholarshipListResult{}, fmt.Errorf("iterate scholarships: %w", err)
	}

	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PageSize - 1) / filter.PageSize
	}

	return ScholarshipListResult{Data: items, Total: total, Page: filter.Page, PageSize: filter.PageSize, TotalPage: totalPage}, nil
}

func (r *DBScholarshipRepository) FindByID(ctx context.Context, id string) (ScholarshipItem, error) {
	countryExpr := countryLabelSQL("u.negara_id")
	query := `
		SELECT
			fo.funding_id,
			fo.nama_beasiswa,
			fo.deskripsi,
			fo.provider,
			fo.tipe_pembiayaan,
			fo.website,
			COALESCE(TO_CHAR(MIN(ap.deadline), 'YYYY-MM-DD'), ''),
			COALESCE(MIN(` + countryExpr + `), '')
		FROM funding_options fo
		LEFT JOIN admission_funding af ON af.funding_id = fo.funding_id
		LEFT JOIN admission_paths ap ON ap.admission_id = af.admission_id
		LEFT JOIN programs p ON p.program_id = ap.program_id
		LEFT JOIN universities u ON u.id = p.university_id
		WHERE fo.funding_id = $1
		GROUP BY fo.funding_id, fo.nama_beasiswa, fo.deskripsi, fo.provider, fo.tipe_pembiayaan, fo.website
	`

	var item ScholarshipItem
	var deskripsi sql.NullString
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&item.Funding.FundingID,
		&item.Funding.NamaBeasiswa,
		&deskripsi,
		&item.Funding.Provider,
		&item.Funding.TipePembiayaan,
		&item.Funding.Website,
		&item.Deadline,
		&item.Negara,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ScholarshipItem{}, errs.ErrScholarshipNotFound
		}
		return ScholarshipItem{}, fmt.Errorf("find scholarship: %w", err)
	}
	if deskripsi.Valid {
		item.Funding.Deskripsi = &deskripsi.String
	}
	item.LinkDaftarURL = item.Funding.Website
	item.IsActive = true

	persyaratan, err := r.findRequirements(ctx, id)
	if err != nil {
		persyaratan = []string{}
	}
	benefit, err := r.findBenefits(ctx, id)
	if err != nil {
		benefit = []string{}
	}
	universitas, err := r.findUniversities(ctx, id)
	if err != nil {
		universitas = []model.University{}
	}

	item.Persyaratan = persyaratan
	item.Benefit = benefit
	item.Universitas = universitas

	return item, nil
}

func (r *DBScholarshipRepository) findRequirements(ctx context.Context, fundingID string) ([]string, error) {
	query := `
		SELECT COALESCE(NULLIF(rc.label, ''), fr.req_catalog_id)
		FROM funding_requirements fr
		LEFT JOIN requirement_catalog rc ON rc.req_catalog_id = fr.req_catalog_id
		WHERE fr.funding_id = $1
		ORDER BY fr.sort_order ASC, fr.funding_req_id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, fundingID)
	if err != nil {
		return nil, fmt.Errorf("find scholarship requirements: %w", err)
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, fmt.Errorf("scan scholarship requirement: %w", err)
		}
		items = append(items, text)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scholarship requirements: %w", err)
	}
	return items, nil
}

func (r *DBScholarshipRepository) findBenefits(ctx context.Context, fundingID string) ([]string, error) {
	query := `
		SELECT
			CASE
				WHEN COALESCE(NULLIF(TRIM(fb.value_text), ''), '') <> ''
					THEN COALESCE(NULLIF(bc.label, ''), fb.benefit_id) || ': ' || fb.value_text
				ELSE COALESCE(NULLIF(bc.label, ''), fb.benefit_id)
			END
		FROM funding_benefits fb
		LEFT JOIN benefit_catalog bc ON bc.benefit_id = fb.benefit_id
		WHERE fb.funding_id = $1
		ORDER BY fb.sort_order ASC, fb.funding_benefit_id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, fundingID)
	if err != nil {
		return nil, fmt.Errorf("find scholarship benefits: %w", err)
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, fmt.Errorf("scan scholarship benefit: %w", err)
		}
		items = append(items, text)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scholarship benefits: %w", err)
	}
	return items, nil
}

func (r *DBScholarshipRepository) findUniversities(ctx context.Context, fundingID string) ([]model.University, error) {
	query := `
		SELECT DISTINCT u.id, u.negara_id, u.nama, u.kota, u.tipe, u.deskripsi, u.website, u.ranking
		FROM admission_funding af
		INNER JOIN admission_paths ap ON ap.admission_id = af.admission_id
		INNER JOIN programs p ON p.program_id = ap.program_id
		INNER JOIN universities u ON u.id = p.university_id
		WHERE af.funding_id = $1
		ORDER BY u.nama ASC
	`
	rows, err := r.db.QueryContext(ctx, query, fundingID)
	if err != nil {
		return nil, fmt.Errorf("find scholarship universities: %w", err)
	}
	defer rows.Close()

	items := make([]model.University, 0)
	for rows.Next() {
		var u model.University
		var ranking sql.NullInt64
		if err := rows.Scan(&u.ID, &u.NegaraID, &u.Nama, &u.Kota, &u.Tipe, &u.Deskripsi, &u.Website, &ranking); err != nil {
			return nil, fmt.Errorf("scan scholarship university: %w", err)
		}
		if ranking.Valid {
			v := int(ranking.Int64)
			u.Ranking = &v
		}
		items = append(items, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scholarship universities: %w", err)
	}
	return items, nil
}

func countryLabelSQL(column string) string {
	return "CASE UPPER(COALESCE(" + column + ", '')) " +
		"WHEN 'ID' THEN 'Indonesia' " +
		"WHEN 'IDN' THEN 'Indonesia' " +
		"WHEN 'GB' THEN 'United Kingdom' " +
		"WHEN 'GBR' THEN 'United Kingdom' " +
		"WHEN 'UK' THEN 'United Kingdom' " +
		"WHEN 'US' THEN 'United States' " +
		"WHEN 'USA' THEN 'United States' " +
		"WHEN 'DE' THEN 'Germany' " +
		"WHEN 'DEU' THEN 'Germany' " +
		"WHEN 'AU' THEN 'Australia' " +
		"WHEN 'AUS' THEN 'Australia' " +
		"WHEN 'JP' THEN 'Japan' " +
		"WHEN 'JPN' THEN 'Japan' " +
		"WHEN 'SG' THEN 'Singapore' " +
		"WHEN 'SGP' THEN 'Singapore' " +
		"ELSE COALESCE(" + column + ", '') END"
}
