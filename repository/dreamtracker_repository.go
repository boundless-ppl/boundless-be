package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"boundless-be/model"

	"github.com/google/uuid"
)

type DreamTrackerSummaryRow struct {
	DreamTrackerID        string
	Title                 string
	Status                string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	ProgramID             string
	ProgramName           string
	UniversityID          string
	UniversityName        string
	AdmissionID           *string
	AdmissionName         string
	AdmissionDeadline     *time.Time
	AdmissionURL          string
	FundingID             *string
	TotalRequirements     int
	CompletedRequirements int
}

type DreamRequirementRow struct {
	DreamReqStatusID string
	ReqCatalogID     string
	RequirementKey   string
	RequirementLabel string
	Category         string
	Status           string
	Notes            *string
	AIStatus         *string
	AIMessages       *string
	Document         *model.Document
}

type DreamMilestoneRow struct {
	DreamMilestoneID string
	Title            string
	Status           string
	DeadlineDate     time.Time
}

type DreamFundingRow struct {
	FundingID    string
	NamaBeasiswa string
	Provider     string
}

type CreateDreamTrackerParams struct {
	UserID            string
	ProgramID         string
	AdmissionID       *string
	FundingID         *string
	Title             string
	Status            string
	SourceType        string
	ReqSubmissionID   *string
	SourceRecResultID *string
}

type DreamRequirementTarget struct {
	DreamReqStatusID string
	DreamTrackerID   string
	UserID           string
	Status           string
}

type DreamTrackerRepository interface {
	ListTrackerSummaries(ctx context.Context, userID string) ([]DreamTrackerSummaryRow, error)
	GetTrackerSummaryByID(ctx context.Context, userID, dreamTrackerID string) (DreamTrackerSummaryRow, error)
	ListRequirements(ctx context.Context, userID, dreamTrackerID string) ([]DreamRequirementRow, error)
	ListMilestones(ctx context.Context, userID, dreamTrackerID string) ([]DreamMilestoneRow, error)
	ListFundings(ctx context.Context, userID, dreamTrackerID string) ([]DreamFundingRow, error)
	ListFundingLinks(ctx context.Context, userID string) ([]struct {
		DreamTrackerID       string
		FundingID            string
		FundingName          string
		ProgramName          string
		UniversityName       string
		Status               string
		CompletionPercentage int
		Title                string
	}, error)
	CreateDreamTracker(ctx context.Context, params CreateDreamTrackerParams) (string, string, error)
	InitializeFundingRequirements(ctx context.Context, dreamTrackerID string, fundingID *string) error
	FindRequirementTarget(ctx context.Context, userID, requirementStatusID string) (DreamRequirementTarget, error)
	CreateDocument(ctx context.Context, doc model.Document) (model.Document, error)
	AttachRequirementDocument(ctx context.Context, target DreamRequirementTarget, documentID string) error
	GetRequirementRowByID(ctx context.Context, userID, requirementStatusID string) (DreamRequirementRow, error)
}

type DBDreamTrackerRepository struct {
	db *sql.DB
}

func NewDreamTrackerRepository(db *sql.DB) *DBDreamTrackerRepository {
	return &DBDreamTrackerRepository{db: db}
}

func (r *DBDreamTrackerRepository) ListTrackerSummaries(ctx context.Context, userID string) ([]DreamTrackerSummaryRow, error) {
	query := `
		SELECT
			dt.dream_tracker_id,
			dt.title,
			dt.status,
			dt.created_at,
			dt.updated_at,
			dt.program_id,
			COALESCE(p.nama, dt.title) AS program_name,
			COALESCE(u.id::text, p.university_id, '') AS university_id,
			COALESCE(NULLIF(p.nama_univ, ''), u.nama, '') AS university_name,
			dt.admission_id::text,
			COALESCE(ap.nama, '') AS admission_name,
			ap.deadline,
			COALESCE(ap.website_url, '') AS admission_url,
			dt.funding_id::text,
			COUNT(drs.dream_req_status_id) AS total_requirements,
			COUNT(*) FILTER (WHERE drs.status IN ('VERIFIED', 'REUSED', 'UPLOADED')) AS completed_requirements
		FROM dream_tracker dt
		LEFT JOIN programs p ON p.program_id = dt.program_id
		LEFT JOIN universities u ON u.id::text = p.university_id
		LEFT JOIN admission_paths ap ON ap.admission_id = dt.admission_id
		LEFT JOIN dream_requirement_status drs ON drs.dream_tracker_id = dt.dream_tracker_id
		WHERE dt.user_id = $1
		GROUP BY
			dt.dream_tracker_id, dt.title, dt.status, dt.created_at, dt.updated_at,
			dt.program_id, p.nama, u.id, p.university_id, p.nama_univ, u.nama,
			dt.admission_id, ap.nama, ap.deadline, ap.website_url, dt.funding_id
		ORDER BY dt.updated_at DESC, dt.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list dream tracker summaries: %w", err)
	}
	defer rows.Close()

	items := make([]DreamTrackerSummaryRow, 0)
	for rows.Next() {
		var row DreamTrackerSummaryRow
		var admissionID, fundingID sql.NullString
		var admissionDeadline sql.NullTime
		if err := rows.Scan(
			&row.DreamTrackerID,
			&row.Title,
			&row.Status,
			&row.CreatedAt,
			&row.UpdatedAt,
			&row.ProgramID,
			&row.ProgramName,
			&row.UniversityID,
			&row.UniversityName,
			&admissionID,
			&row.AdmissionName,
			&admissionDeadline,
			&row.AdmissionURL,
			&fundingID,
			&row.TotalRequirements,
			&row.CompletedRequirements,
		); err != nil {
			return nil, fmt.Errorf("scan dream tracker summary: %w", err)
		}
		if admissionID.Valid {
			row.AdmissionID = &admissionID.String
		}
		if fundingID.Valid {
			row.FundingID = &fundingID.String
		}
		if admissionDeadline.Valid {
			row.AdmissionDeadline = &admissionDeadline.Time
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (r *DBDreamTrackerRepository) GetTrackerSummaryByID(ctx context.Context, userID, dreamTrackerID string) (DreamTrackerSummaryRow, error) {
	rows, err := r.ListTrackerSummaries(ctx, userID)
	if err != nil {
		return DreamTrackerSummaryRow{}, err
	}
	for _, row := range rows {
		if row.DreamTrackerID == dreamTrackerID {
			return row, nil
		}
	}
	return DreamTrackerSummaryRow{}, sql.ErrNoRows
}

func (r *DBDreamTrackerRepository) ListRequirements(ctx context.Context, userID, dreamTrackerID string) ([]DreamRequirementRow, error) {
	query := `
		SELECT
			drs.dream_req_status_id,
			drs.req_catalog_id,
			rc.key,
			rc.label,
			rc.kategori,
			drs.status,
			drs.notes,
			drs.ai_status,
			drs.ai_messages,
			d.document_id,
			d.document_type,
			d.original_filename,
			d.public_url,
			d.mime_type,
			d.uploaded_at
		FROM dream_requirement_status drs
		JOIN dream_tracker dt ON dt.dream_tracker_id = drs.dream_tracker_id
		JOIN requirement_catalog rc ON rc.req_catalog_id = drs.req_catalog_id
		LEFT JOIN documents d ON d.document_id = drs.document_id
		WHERE dt.user_id = $1 AND drs.dream_tracker_id = $2
		ORDER BY rc.label ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID, dreamTrackerID)
	if err != nil {
		return nil, fmt.Errorf("list dream tracker requirements: %w", err)
	}
	defer rows.Close()

	items := make([]DreamRequirementRow, 0)
	for rows.Next() {
		var row DreamRequirementRow
		var notes, aiStatus, aiMessages sql.NullString
		var docID, docType, fileName, publicURL, mimeType sql.NullString
		var uploadedAt sql.NullTime
		if err := rows.Scan(
			&row.DreamReqStatusID,
			&row.ReqCatalogID,
			&row.RequirementKey,
			&row.RequirementLabel,
			&row.Category,
			&row.Status,
			&notes,
			&aiStatus,
			&aiMessages,
			&docID,
			&docType,
			&fileName,
			&publicURL,
			&mimeType,
			&uploadedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dream requirement: %w", err)
		}
		if notes.Valid {
			row.Notes = &notes.String
		}
		if aiStatus.Valid {
			row.AIStatus = &aiStatus.String
		}
		if aiMessages.Valid {
			row.AIMessages = &aiMessages.String
		}
		if docID.Valid {
			row.Document = &model.Document{
				DocumentID:       docID.String,
				DocumentType:     model.DocumentType(docType.String),
				OriginalFilename: fileName.String,
				PublicURL:        publicURL.String,
				MIMEType:         mimeType.String,
				UploadedAt:       uploadedAt.Time,
			}
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (r *DBDreamTrackerRepository) ListMilestones(ctx context.Context, userID, dreamTrackerID string) ([]DreamMilestoneRow, error) {
	query := `
		SELECT dkm.dream_milestone_id, dkm.title, dkm.status, dkm.deadline_date
		FROM dream_key_milestones dkm
		JOIN dream_tracker dt ON dt.dream_tracker_id = dkm.dream_tracker_id
		WHERE dt.user_id = $1 AND dkm.dream_tracker_id = $2
		ORDER BY dkm.deadline_date ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID, dreamTrackerID)
	if err != nil {
		return nil, fmt.Errorf("list dream milestones: %w", err)
	}
	defer rows.Close()

	items := make([]DreamMilestoneRow, 0)
	for rows.Next() {
		var row DreamMilestoneRow
		if err := rows.Scan(&row.DreamMilestoneID, &row.Title, &row.Status, &row.DeadlineDate); err != nil {
			return nil, fmt.Errorf("scan dream milestone: %w", err)
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (r *DBDreamTrackerRepository) ListFundings(ctx context.Context, userID, dreamTrackerID string) ([]DreamFundingRow, error) {
	query := `
		SELECT DISTINCT fo.funding_id, fo.nama_beasiswa, fo.provider
		FROM dream_tracker dt
		LEFT JOIN admission_funding af ON af.admission_id = dt.admission_id
		LEFT JOIN funding_options fo ON fo.funding_id = COALESCE(dt.funding_id, af.funding_id)
		WHERE dt.user_id = $1 AND dt.dream_tracker_id = $2 AND fo.funding_id IS NOT NULL
		ORDER BY fo.nama_beasiswa ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID, dreamTrackerID)
	if err != nil {
		return nil, fmt.Errorf("list dream fundings: %w", err)
	}
	defer rows.Close()

	items := make([]DreamFundingRow, 0)
	for rows.Next() {
		var row DreamFundingRow
		if err := rows.Scan(&row.FundingID, &row.NamaBeasiswa, &row.Provider); err != nil {
			return nil, fmt.Errorf("scan dream funding: %w", err)
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (r *DBDreamTrackerRepository) ListFundingLinks(ctx context.Context, userID string) ([]struct {
	DreamTrackerID       string
	FundingID            string
	FundingName          string
	ProgramName          string
	UniversityName       string
	Status               string
	CompletionPercentage int
	Title                string
}, error) {
	query := `
		SELECT
			dt.dream_tracker_id,
			fo.funding_id,
			fo.nama_beasiswa,
			COALESCE(p.nama, dt.title) AS program_name,
			COALESCE(NULLIF(p.nama_univ, ''), u.nama, '') AS university_name,
			dt.status,
			CASE
				WHEN COUNT(drs.dream_req_status_id) = 0 THEN 0
				ELSE ROUND((COUNT(*) FILTER (WHERE drs.status IN ('VERIFIED', 'REUSED', 'UPLOADED'))::numeric / COUNT(drs.dream_req_status_id)::numeric) * 100)::int
			END AS completion_percentage,
			dt.title
		FROM dream_tracker dt
		LEFT JOIN programs p ON p.program_id = dt.program_id
		LEFT JOIN universities u ON u.id::text = p.university_id
		LEFT JOIN admission_funding af ON af.admission_id = dt.admission_id
		JOIN funding_options fo ON fo.funding_id = COALESCE(dt.funding_id, af.funding_id)
		LEFT JOIN dream_requirement_status drs ON drs.dream_tracker_id = dt.dream_tracker_id
		WHERE dt.user_id = $1
		GROUP BY dt.dream_tracker_id, fo.funding_id, fo.nama_beasiswa, p.nama, dt.title, p.nama_univ, u.nama, dt.status
		ORDER BY fo.nama_beasiswa ASC, dt.updated_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list dream funding links: %w", err)
	}
	defer rows.Close()

	items := make([]struct {
		DreamTrackerID       string
		FundingID            string
		FundingName          string
		ProgramName          string
		UniversityName       string
		Status               string
		CompletionPercentage int
		Title                string
	}, 0)
	for rows.Next() {
		var row struct {
			DreamTrackerID       string
			FundingID            string
			FundingName          string
			ProgramName          string
			UniversityName       string
			Status               string
			CompletionPercentage int
			Title                string
		}
		if err := rows.Scan(&row.DreamTrackerID, &row.FundingID, &row.FundingName, &row.ProgramName, &row.UniversityName, &row.Status, &row.CompletionPercentage, &row.Title); err != nil {
			return nil, fmt.Errorf("scan dream funding link: %w", err)
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (r *DBDreamTrackerRepository) CreateDreamTracker(ctx context.Context, params CreateDreamTrackerParams) (string, string, error) {
	id := uuid.NewString()
	status := params.Status
	if status == "" {
		status = "ACTIVE"
	}
	query := `
		INSERT INTO dream_tracker (
			dream_tracker_id, user_id, program_id, admission_id, funding_id, title, status,
			created_at, updated_at, source_type, req_submission_id, source_rec_result_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW(),$8,$9,$10)
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		id,
		params.UserID,
		params.ProgramID,
		params.AdmissionID,
		params.FundingID,
		params.Title,
		status,
		params.SourceType,
		params.ReqSubmissionID,
		params.SourceRecResultID,
	)
	if err != nil {
		return "", "", fmt.Errorf("create dream tracker: %w", err)
	}
	return id, status, nil
}

func (r *DBDreamTrackerRepository) InitializeFundingRequirements(ctx context.Context, dreamTrackerID string, fundingID *string) error {
	if fundingID == nil || *fundingID == "" {
		return nil
	}

	query := `
		INSERT INTO dream_requirement_status (
			dream_req_status_id, dream_tracker_id, document_id, req_catalog_id, status, notes, ai_status, ai_messages, created_at
		)
		SELECT gen_random_uuid(), $1, NULL, fr.req_catalog_id, 'NOT_UPLOADED', NULL, NULL, NULL, NOW()
		FROM funding_requirements fr
		WHERE fr.funding_id = $2
		  AND NOT EXISTS (
		    SELECT 1 FROM dream_requirement_status drs
		    WHERE drs.dream_tracker_id = $1 AND drs.req_catalog_id = fr.req_catalog_id
		  )
	`
	_, err := r.db.ExecContext(ctx, query, dreamTrackerID, *fundingID)
	return err
}

func (r *DBDreamTrackerRepository) FindRequirementTarget(ctx context.Context, userID, requirementStatusID string) (DreamRequirementTarget, error) {
	query := `
		SELECT drs.dream_req_status_id, drs.dream_tracker_id, dt.user_id, drs.status
		FROM dream_requirement_status drs
		JOIN dream_tracker dt ON dt.dream_tracker_id = drs.dream_tracker_id
		WHERE drs.dream_req_status_id = $1 AND dt.user_id = $2
	`
	var target DreamRequirementTarget
	err := r.db.QueryRowContext(ctx, query, requirementStatusID, userID).
		Scan(&target.DreamReqStatusID, &target.DreamTrackerID, &target.UserID, &target.Status)
	if err != nil {
		return DreamRequirementTarget{}, err
	}
	return target, nil
}

func (r *DBDreamTrackerRepository) CreateDocument(ctx context.Context, doc model.Document) (model.Document, error) {
	query := `
		INSERT INTO documents
		(document_id, user_id, original_filename, storage_path, public_url, mime_type, size_bytes, document_type, uploaded_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		doc.DocumentID,
		doc.UserID,
		doc.OriginalFilename,
		doc.StoragePath,
		doc.PublicURL,
		doc.MIMEType,
		doc.SizeBytes,
		doc.DocumentType,
		doc.UploadedAt,
	)
	if err != nil {
		return model.Document{}, fmt.Errorf("insert dream tracker document: %w", err)
	}
	return doc, nil
}

func (r *DBDreamTrackerRepository) AttachRequirementDocument(ctx context.Context, target DreamRequirementTarget, documentID string) error {
	query := `
		UPDATE dream_requirement_status
		SET document_id = $3, status = 'UPLOADED', notes = NULL, ai_status = 'PENDING', ai_messages = NULL
		WHERE dream_req_status_id = $1 AND dream_tracker_id = $2
	`
	_, err := r.db.ExecContext(ctx, query, target.DreamReqStatusID, target.DreamTrackerID, documentID)
	return err
}

func (r *DBDreamTrackerRepository) GetRequirementRowByID(ctx context.Context, userID, requirementStatusID string) (DreamRequirementRow, error) {
	query := `
		SELECT
			drs.dream_req_status_id,
			drs.req_catalog_id,
			rc.key,
			rc.label,
			rc.kategori,
			drs.status,
			drs.notes,
			drs.ai_status,
			drs.ai_messages,
			d.document_id,
			d.document_type,
			d.original_filename,
			d.public_url,
			d.mime_type,
			d.uploaded_at
		FROM dream_requirement_status drs
		JOIN dream_tracker dt ON dt.dream_tracker_id = drs.dream_tracker_id
		JOIN requirement_catalog rc ON rc.req_catalog_id = drs.req_catalog_id
		LEFT JOIN documents d ON d.document_id = drs.document_id
		WHERE dt.user_id = $1 AND drs.dream_req_status_id = $2
	`
	rows, err := r.db.QueryContext(ctx, query, userID, requirementStatusID)
	if err != nil {
		return DreamRequirementRow{}, err
	}
	defer rows.Close()
	list, err := r.ListRequirements(ctx, userID, "")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		_ = list
	}
	var row DreamRequirementRow
	var notes, aiStatus, aiMessages sql.NullString
	var docID, docType, fileName, publicURL, mimeType sql.NullString
	var uploadedAt sql.NullTime
	if !rows.Next() {
		return DreamRequirementRow{}, sql.ErrNoRows
	}
	if err := rows.Scan(
		&row.DreamReqStatusID,
		&row.ReqCatalogID,
		&row.RequirementKey,
		&row.RequirementLabel,
		&row.Category,
		&row.Status,
		&notes,
		&aiStatus,
		&aiMessages,
		&docID,
		&docType,
		&fileName,
		&publicURL,
		&mimeType,
		&uploadedAt,
	); err != nil {
		return DreamRequirementRow{}, err
	}
	if notes.Valid {
		row.Notes = &notes.String
	}
	if aiStatus.Valid {
		row.AIStatus = &aiStatus.String
	}
	if aiMessages.Valid {
		row.AIMessages = &aiMessages.String
	}
	if docID.Valid {
		row.Document = &model.Document{
			DocumentID:       docID.String,
			DocumentType:     model.DocumentType(docType.String),
			OriginalFilename: fileName.String,
			PublicURL:        publicURL.String,
			MIMEType:         mimeType.String,
			UploadedAt:       uploadedAt.Time,
		}
	}
	return row, nil
}
