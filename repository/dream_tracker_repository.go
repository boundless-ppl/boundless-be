package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"boundless-be/errs"
	"boundless-be/model"

	"github.com/google/uuid"
)

type DreamTrackerDetail struct {
	DreamTracker model.DreamTracker
	Summary      model.DreamTrackerSummary
	ProgramInfo  model.DreamTrackerProgramInfo
	Requirements []model.DreamRequirementDetail
	Milestones   []model.DreamKeyMilestone
	Fundings     []model.DreamTrackerFundingOption
}

type DreamTrackerSeed struct {
	ProgramID   string
	Title       string
	AdmissionID *string
	FundingID   *string
}

type DreamTrackerRepository interface {
	CreateDreamTracker(ctx context.Context, tracker model.DreamTracker) (model.DreamTracker, error)
	FindDreamTrackersByUser(ctx context.Context, userID string) ([]model.DreamTracker, error)
	FindDreamTrackerDetail(ctx context.Context, dreamTrackerID, userID string) (DreamTrackerDetail, error)
	ResolveDreamTrackerSeed(ctx context.Context, programID *string, sourceRecResultID *string) (DreamTrackerSeed, error)
	CreateDocument(ctx context.Context, doc model.Document) (model.Document, error)
	FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error)
	FindReusableDocumentByUserAndType(ctx context.Context, userID, documentType string) (model.Document, bool, error)
	FindDreamRequirementStatusByIDAndUser(ctx context.Context, dreamReqStatusID, userID string) (model.DreamRequirementStatus, error)
	UpdateDreamRequirementStatus(ctx context.Context, requirement model.DreamRequirementStatus) error
}

type DBDreamTrackerRepository struct {
	db *sql.DB
}

func NewDreamTrackerRepository(db *sql.DB) *DBDreamTrackerRepository {
	return &DBDreamTrackerRepository{db: db}
}

func (r *DBDreamTrackerRepository) CreateDreamTracker(ctx context.Context, tracker model.DreamTracker) (model.DreamTracker, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.DreamTracker{}, fmt.Errorf("begin tx create dream tracker: %w", err)
	}
	defer tx.Rollback()

	insertTrackerQuery := `
		INSERT INTO dream_tracker
		(dream_tracker_id, user_id, program_id, admission_id, funding_id, title, status, created_at, updated_at, source_type, req_submission_id, source_rec_result_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`
	_, err = tx.ExecContext(
		ctx,
		insertTrackerQuery,
		tracker.DreamTrackerID,
		tracker.UserID,
		tracker.ProgramID,
		nullString(tracker.AdmissionID),
		nullString(tracker.FundingID),
		tracker.Title,
		tracker.Status,
		tracker.CreatedAt,
		tracker.UpdatedAt,
		tracker.SourceType,
		nullString(tracker.ReqSubmissionID),
		nullString(tracker.SourceRecResultID),
	)
	if err != nil {
		return model.DreamTracker{}, fmt.Errorf("insert dream tracker: %w", err)
	}

	if err := r.seedDreamRequirementStatuses(ctx, tx, tracker); err != nil {
		return model.DreamTracker{}, err
	}

	if err := tx.Commit(); err != nil {
		return model.DreamTracker{}, fmt.Errorf("commit create dream tracker: %w", err)
	}

	return tracker, nil
}

func (r *DBDreamTrackerRepository) FindDreamTrackerDetail(ctx context.Context, dreamTrackerID, userID string) (DreamTrackerDetail, error) {
	var detail DreamTrackerDetail
	tracker, err := r.findDreamTracker(ctx, dreamTrackerID, userID)
	if err != nil {
		return DreamTrackerDetail{}, err
	}

	// Treat enrichment queries as best-effort so one inconsistent related row
	// does not break the entire dream tracker dashboard for the user.
	requirements, err := r.findDreamRequirementDetails(ctx, dreamTrackerID)
	if err != nil {
		requirements = []model.DreamRequirementDetail{}
	}
	programInfo, err := r.findDreamTrackerProgramInfo(ctx, tracker)
	if err != nil {
		programInfo = model.DreamTrackerProgramInfo{ProgramID: tracker.ProgramID}
	}
	milestones, err := r.findDreamKeyMilestones(ctx, dreamTrackerID)
	if err != nil {
		milestones = []model.DreamKeyMilestone{}
	}
	fundings, err := r.findDreamTrackerFundings(ctx, tracker)
	if err != nil {
		fundings = []model.DreamTrackerFundingOption{}
	}
	detail.DreamTracker = tracker
	detail.ProgramInfo = programInfo
	detail.Requirements = requirements
	detail.Milestones = milestones
	detail.Fundings = fundings
	return detail, nil
}

func (r *DBDreamTrackerRepository) FindDreamTrackersByUser(ctx context.Context, userID string) ([]model.DreamTracker, error) {
	query := `
		SELECT dream_tracker_id, user_id, program_id, admission_id, funding_id, title, status, created_at, updated_at, source_type, req_submission_id, source_rec_result_id
		FROM dream_tracker
		WHERE user_id = $1
		ORDER BY updated_at DESC, created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("find dream trackers by user: %w", err)
	}
	defer rows.Close()

	items := make([]model.DreamTracker, 0)
	for rows.Next() {
		item, scanErr := scanDreamTracker(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dream trackers by user: %w", err)
	}

	return items, nil
}

func (r *DBDreamTrackerRepository) ResolveDreamTrackerSeed(ctx context.Context, programID *string, sourceRecResultID *string) (DreamTrackerSeed, error) {
	if programID == nil && sourceRecResultID == nil {
		return DreamTrackerSeed{}, errs.ErrInvalidInput
	}

	query := `
		WITH base AS (
			SELECT
				COALESCE(NULLIF(rr.program_id, ''), $1) AS program_id,
				COALESCE(NULLIF(rr.program_name, ''), p.nama) AS program_name,
				COALESCE(NULLIF(rr.university_name, ''), u.nama) AS university_name
			FROM programs p
			JOIN universities u ON u.id = p.university_id
			LEFT JOIN recommendation_results rr ON rr.rec_result_id = $2
			WHERE p.program_id = COALESCE(NULLIF(rr.program_id, ''), $1)
			LIMIT 1
		)
		SELECT
			base.program_id,
			COALESCE(NULLIF(base.program_name, ''), NULLIF(base.university_name, ''), base.program_id) AS title,
			ap.admission_id,
			af.funding_id
		FROM base
		LEFT JOIN LATERAL (
			SELECT ap.admission_id
			FROM admission_paths ap
			WHERE ap.program_id = base.program_id
			ORDER BY ap.deadline ASC NULLS LAST, ap.admission_id ASC
			LIMIT 1
		) ap ON TRUE
		LEFT JOIN LATERAL (
			SELECT af.funding_id
			FROM admission_funding af
			INNER JOIN funding_options fo ON fo.funding_id = af.funding_id
			WHERE ap.admission_id IS NOT NULL AND af.admission_id = ap.admission_id
			ORDER BY
				CASE fo.tipe_pembiayaan
					WHEN 'SCHOLARSHIP' THEN 0
					WHEN 'ASSISTANTSHIP' THEN 1
					WHEN 'SPONSORSHIP' THEN 2
					WHEN 'LOAN' THEN 3
					ELSE 4
				END,
				fo.nama_beasiswa ASC,
				af.admission_funding_id ASC
			LIMIT 1
		) af ON TRUE
	`

	var seed DreamTrackerSeed
	var admissionID sql.NullString
	var fundingID sql.NullString
	err := r.db.QueryRowContext(
		ctx,
		query,
		nullString(programID),
		nullString(sourceRecResultID),
	).Scan(&seed.ProgramID, &seed.Title, &admissionID, &fundingID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DreamTrackerSeed{}, errs.ErrInvalidInput
		}
		return DreamTrackerSeed{}, fmt.Errorf("resolve dream tracker seed: %w", err)
	}
	assignNullString(&seed.AdmissionID, admissionID)
	assignNullString(&seed.FundingID, fundingID)
	return seed, nil
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
		return model.Document{}, fmt.Errorf("insert document: %w", err)
	}
	return doc, nil
}

func (r *DBDreamTrackerRepository) FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error) {
	query := `
		SELECT document_id, user_id, nama, original_filename, storage_path, dokumen_url, public_url, mime_type, size_bytes, dokumen_size_kb, document_type, uploaded_at
		FROM documents
		WHERE document_id = $1 AND user_id = $2
	`

	var doc model.Document
	var nama sql.NullString
	var originalFilename sql.NullString
	var storagePath sql.NullString
	var dokumenURL sql.NullString
	var publicURL sql.NullString
	var sizeBytes sql.NullInt64
	var dokumenSizeKB sql.NullInt64
	var documentType sql.NullString

	err := r.db.QueryRowContext(ctx, query, documentID, userID).Scan(
		&doc.DocumentID,
		&doc.UserID,
		&nama,
		&originalFilename,
		&storagePath,
		&dokumenURL,
		&publicURL,
		&doc.MIMEType,
		&sizeBytes,
		&dokumenSizeKB,
		&documentType,
		&doc.UploadedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Document{}, errs.ErrDocumentNotFound
		}
		return model.Document{}, fmt.Errorf("find document by id and user: %w", err)
	}

	if nama.Valid {
		doc.Nama = nama.String
	}
	if originalFilename.Valid {
		doc.OriginalFilename = originalFilename.String
	}
	if storagePath.Valid {
		doc.StoragePath = storagePath.String
	}
	if dokumenURL.Valid {
		doc.DokumenURL = dokumenURL.String
	}
	if publicURL.Valid {
		doc.PublicURL = publicURL.String
	}
	if sizeBytes.Valid {
		doc.SizeBytes = sizeBytes.Int64
	}
	if dokumenSizeKB.Valid {
		doc.DokumenSizeKB = dokumenSizeKB.Int64
	}
	if documentType.Valid {
		doc.DocumentType = model.DocumentType(documentType.String)
	}

	return doc, nil
}

func (r *DBDreamTrackerRepository) FindReusableDocumentByUserAndType(ctx context.Context, userID, documentType string) (model.Document, bool, error) {
	query := `
		SELECT d.document_id, d.user_id, d.original_filename, d.storage_path, d.public_url, d.mime_type, d.size_bytes, d.document_type, d.uploaded_at
		FROM documents d
		INNER JOIN dream_requirement_status drs ON drs.document_id = d.document_id
		INNER JOIN dream_tracker dt ON dt.dream_tracker_id = drs.dream_tracker_id
		WHERE d.user_id = $1
		  AND UPPER(d.document_type) = UPPER($2)
		  AND dt.user_id = $1
		  AND drs.status IN ('VERIFIED', 'VERIFIED_WITH_WARNING', 'REUSED')
		ORDER BY d.uploaded_at DESC
		LIMIT 1
	`
	var doc model.Document
	err := r.db.QueryRowContext(ctx, query, userID, documentType).Scan(
		&doc.DocumentID,
		&doc.UserID,
		&doc.OriginalFilename,
		&doc.StoragePath,
		&doc.PublicURL,
		&doc.MIMEType,
		&doc.SizeBytes,
		&doc.DocumentType,
		&doc.UploadedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Document{}, false, nil
		}
		return model.Document{}, false, fmt.Errorf("find reusable document by user and type: %w", err)
	}
	return doc, true, nil
}

func (r *DBDreamTrackerRepository) FindDreamRequirementStatusByIDAndUser(ctx context.Context, dreamReqStatusID, userID string) (model.DreamRequirementStatus, error) {
	query := `
		SELECT drs.dream_req_status_id, drs.dream_tracker_id, drs.document_id, drs.req_catalog_id, drs.status, drs.notes, drs.ai_status, drs.ai_messages, drs.created_at
		FROM dream_requirement_status drs
		INNER JOIN dream_tracker dt ON dt.dream_tracker_id = drs.dream_tracker_id
		WHERE drs.dream_req_status_id = $1 AND dt.user_id = $2
	`

	var item model.DreamRequirementStatus
	var documentID sql.NullString
	var notes sql.NullString
	var aiStatus sql.NullString
	var aiMessages sql.NullString

	err := r.db.QueryRowContext(ctx, query, dreamReqStatusID, userID).Scan(
		&item.DreamReqStatusID,
		&item.DreamTrackerID,
		&documentID,
		&item.ReqCatalogID,
		&item.Status,
		&notes,
		&aiStatus,
		&aiMessages,
		&item.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.DreamRequirementStatus{}, errs.ErrDreamRequirementNotFound
		}
		return model.DreamRequirementStatus{}, fmt.Errorf("find dream requirement status: %w", err)
	}

	if documentID.Valid {
		item.DocumentID = &documentID.String
	}
	if notes.Valid {
		item.Notes = &notes.String
	}
	if aiStatus.Valid {
		item.AIStatus = &aiStatus.String
	}
	if aiMessages.Valid {
		item.AIMessages = &aiMessages.String
	}

	return item, nil
}

func (r *DBDreamTrackerRepository) UpdateDreamRequirementStatus(ctx context.Context, requirement model.DreamRequirementStatus) error {
	query := `
		UPDATE dream_requirement_status
		SET document_id = $2, status = $3, notes = $4, ai_status = $5, ai_messages = $6
		WHERE dream_req_status_id = $1
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		requirement.DreamReqStatusID,
		nullString(requirement.DocumentID),
		requirement.Status,
		nullString(requirement.Notes),
		nullString(requirement.AIStatus),
		nullString(requirement.AIMessages),
	)
	if err != nil {
		return fmt.Errorf("update dream requirement status: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows update dream requirement status: %w", err)
	}
	if affected == 0 {
		return errs.ErrDreamRequirementNotFound
	}

	return nil
}

func (r *DBDreamTrackerRepository) seedDreamRequirementStatuses(ctx context.Context, tx *sql.Tx, tracker model.DreamTracker) error {
	if tracker.FundingID == nil {
		return nil
	}

	reqCatalogIDs, err := findFundingRequirementIDs(ctx, tx, *tracker.FundingID)
	if err != nil {
		return err
	}

	for _, reqCatalogID := range reqCatalogIDs {
		if err := insertDreamRequirementStatus(ctx, tx, tracker, reqCatalogID); err != nil {
			return err
		}
	}
	return nil
}

func findFundingRequirementIDs(ctx context.Context, tx *sql.Tx, fundingID string) ([]string, error) {
	reqQuery := `
		SELECT req_catalog_id
		FROM funding_requirements
		WHERE funding_id = $1
		ORDER BY sort_order ASC, funding_req_id ASC
	`
	rows, err := tx.QueryContext(ctx, reqQuery, fundingID)
	if err != nil {
		return nil, fmt.Errorf("find funding requirements: %w", err)
	}
	defer rows.Close()

	reqCatalogIDs := make([]string, 0)
	for rows.Next() {
		var reqCatalogID string
		if err := rows.Scan(&reqCatalogID); err != nil {
			return nil, fmt.Errorf("scan funding requirement: %w", err)
		}
		reqCatalogIDs = append(reqCatalogIDs, reqCatalogID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate funding requirements: %w", err)
	}
	return reqCatalogIDs, nil
}

func insertDreamRequirementStatus(ctx context.Context, tx *sql.Tx, tracker model.DreamTracker, reqCatalogID string) error {
	insertRequirementQuery := `
		INSERT INTO dream_requirement_status
		(dream_req_status_id, dream_tracker_id, document_id, req_catalog_id, status, notes, ai_status, ai_messages, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`
	_, err := tx.ExecContext(
		ctx,
		insertRequirementQuery,
		uuid.NewString(),
		tracker.DreamTrackerID,
		nil,
		reqCatalogID,
		model.DreamRequirementStatusNotUploaded,
		nil,
		nil,
		nil,
		tracker.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert dream requirement status: %w", err)
	}
	return nil
}

func (r *DBDreamTrackerRepository) findDreamTracker(ctx context.Context, dreamTrackerID, userID string) (model.DreamTracker, error) {
	query := `
		SELECT dream_tracker_id, user_id, program_id, admission_id, funding_id, title, status, created_at, updated_at, source_type, req_submission_id, source_rec_result_id
		FROM dream_tracker
		WHERE dream_tracker_id = $1 AND user_id = $2
	`

	tracker, err := scanDreamTrackerRow(r.db.QueryRowContext(ctx, query, dreamTrackerID, userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.DreamTracker{}, errs.ErrDreamTrackerNotFound
		}
		return model.DreamTracker{}, fmt.Errorf("find dream tracker detail: %w", err)
	}

	return tracker, nil
}

func scanDreamTracker(scanner interface {
	Scan(dest ...any) error
}) (model.DreamTracker, error) {
	item, err := scanDreamTrackerRow(scanner)
	if err != nil {
		return model.DreamTracker{}, fmt.Errorf("scan dream tracker: %w", err)
	}
	return item, nil
}

func scanDreamTrackerRow(scanner interface {
	Scan(dest ...any) error
}) (model.DreamTracker, error) {
	var tracker model.DreamTracker
	var admissionID sql.NullString
	var fundingID sql.NullString
	var reqSubmissionID sql.NullString
	var sourceRecResultID sql.NullString

	err := scanner.Scan(
		&tracker.DreamTrackerID,
		&tracker.UserID,
		&tracker.ProgramID,
		&admissionID,
		&fundingID,
		&tracker.Title,
		&tracker.Status,
		&tracker.CreatedAt,
		&tracker.UpdatedAt,
		&tracker.SourceType,
		&reqSubmissionID,
		&sourceRecResultID,
	)
	if err != nil {
		return model.DreamTracker{}, err
	}

	assignNullString(&tracker.AdmissionID, admissionID)
	assignNullString(&tracker.FundingID, fundingID)
	assignNullString(&tracker.ReqSubmissionID, reqSubmissionID)
	assignNullString(&tracker.SourceRecResultID, sourceRecResultID)

	return tracker, nil
}

func (r *DBDreamTrackerRepository) findDreamRequirementDetails(ctx context.Context, dreamTrackerID string) ([]model.DreamRequirementDetail, error) {
	reqQuery := `
		SELECT drs.dream_req_status_id, drs.dream_tracker_id, drs.document_id, drs.req_catalog_id, drs.status, drs.notes, drs.ai_status, drs.ai_messages, drs.created_at,
		       rc.key, rc.label, rc.kategori, rc.deskripsi, COALESCE(fr.is_required, FALSE),
		       d.original_filename, d.public_url, d.mime_type, d.size_bytes, d.document_type, d.uploaded_at
		FROM dream_requirement_status drs
		INNER JOIN dream_tracker dt ON dt.dream_tracker_id = drs.dream_tracker_id
		INNER JOIN requirement_catalog rc ON rc.req_catalog_id = drs.req_catalog_id
		LEFT JOIN funding_requirements fr ON fr.funding_id = dt.funding_id AND fr.req_catalog_id = drs.req_catalog_id
		LEFT JOIN documents d ON d.document_id = drs.document_id
		WHERE drs.dream_tracker_id = $1
		ORDER BY drs.created_at ASC, drs.dream_req_status_id ASC
	`
	rows, err := r.db.QueryContext(ctx, reqQuery, dreamTrackerID)
	if err != nil {
		return nil, fmt.Errorf("find dream requirement details: %w", err)
	}
	defer rows.Close()

	items := make([]model.DreamRequirementDetail, 0)
	for rows.Next() {
		item, err := scanDreamRequirementDetail(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dream requirement details: %w", err)
	}
	return items, nil
}

func scanDreamRequirementDetail(scanner interface {
	Scan(dest ...any) error
}) (model.DreamRequirementDetail, error) {
	var item model.DreamRequirementDetail
	var documentID sql.NullString
	var notes sql.NullString
	var aiStatus sql.NullString
	var aiMessages sql.NullString
	var requirementDescription sql.NullString
	var originalFilename sql.NullString
	var publicURL sql.NullString
	var mimeType sql.NullString
	var sizeBytes sql.NullInt64
	var documentType sql.NullString
	var uploadedAt sql.NullTime

	if err := scanner.Scan(
		&item.DreamRequirementStatus.DreamReqStatusID,
		&item.DreamRequirementStatus.DreamTrackerID,
		&documentID,
		&item.DreamRequirementStatus.ReqCatalogID,
		&item.DreamRequirementStatus.Status,
		&notes,
		&aiStatus,
		&aiMessages,
		&item.DreamRequirementStatus.CreatedAt,
		&item.RequirementKey,
		&item.RequirementLabel,
		&item.RequirementCategory,
		&requirementDescription,
		&item.IsRequired,
		&originalFilename,
		&publicURL,
		&mimeType,
		&sizeBytes,
		&documentType,
		&uploadedAt,
	); err != nil {
		return model.DreamRequirementDetail{}, fmt.Errorf("scan dream requirement detail: %w", err)
	}

	assignNullString(&item.DreamRequirementStatus.DocumentID, documentID)
	assignNullString(&item.DreamRequirementStatus.Notes, notes)
	assignNullString(&item.DreamRequirementStatus.AIStatus, aiStatus)
	assignNullString(&item.DreamRequirementStatus.AIMessages, aiMessages)
	assignNullString(&item.RequirementDescription, requirementDescription)
	item.ActionLabel, item.CanUpload, item.NeedsReupload = buildRequirementAction(item.DreamRequirementStatus.Status)
	if item.DocumentID != nil {
		item.Document = &model.Document{
			DocumentID: *item.DocumentID,
		}
		if originalFilename.Valid {
			item.Document.OriginalFilename = originalFilename.String
		}
		if publicURL.Valid {
			item.Document.PublicURL = publicURL.String
		}
		if mimeType.Valid {
			item.Document.MIMEType = mimeType.String
		}
		if sizeBytes.Valid {
			item.Document.SizeBytes = sizeBytes.Int64
		}
		if documentType.Valid {
			item.Document.DocumentType = model.DocumentType(documentType.String)
		}
		if uploadedAt.Valid {
			item.Document.UploadedAt = uploadedAt.Time
		}
	}

	return item, nil
}

func (r *DBDreamTrackerRepository) findDreamTrackerProgramInfo(ctx context.Context, tracker model.DreamTracker) (model.DreamTrackerProgramInfo, error) {
	info := model.DreamTrackerProgramInfo{ProgramID: tracker.ProgramID}

	query := `
		SELECT
			ap.nama,
			ap.intake,
			ap.deadline,
			ap.website_url,
			COALESCE(NULLIF(rr.program_name, ''), p.nama),
			COALESCE(NULLIF(rr.university_name, ''), u.nama)
		FROM dream_tracker dt
		LEFT JOIN admission_paths ap ON ap.admission_id = dt.admission_id
		LEFT JOIN recommendation_results rr ON rr.rec_result_id = dt.source_rec_result_id
		LEFT JOIN programs p ON p.program_id = dt.program_id
		LEFT JOIN universities u ON u.id = p.university_id
		WHERE dt.dream_tracker_id = $1
	`

	var admissionName sql.NullString
	var intake sql.NullString
	var admissionDeadline sql.NullTime
	var admissionURL sql.NullString
	var programName sql.NullString
	var universityName sql.NullString

	err := r.db.QueryRowContext(ctx, query, tracker.DreamTrackerID).Scan(
		&admissionName,
		&intake,
		&admissionDeadline,
		&admissionURL,
		&programName,
		&universityName,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return info, nil
		}
		return model.DreamTrackerProgramInfo{}, fmt.Errorf("find dream tracker program info: %w", err)
	}

	assignNullString(&info.AdmissionName, admissionName)
	assignNullString(&info.Intake, intake)
	assignNullTime(&info.AdmissionDeadline, admissionDeadline)
	assignNullString(&info.AdmissionURL, admissionURL)
	assignNullString(&info.ProgramName, programName)
	assignNullString(&info.UniversityName, universityName)
	return info, nil
}

func (r *DBDreamTrackerRepository) findDreamKeyMilestones(ctx context.Context, dreamTrackerID string) ([]model.DreamKeyMilestone, error) {
	query := `
		SELECT dream_milestone_id, dream_tracker_id, title, description, deadline_date, is_required, status, created_at, updated_at
		FROM dream_key_milestones
		WHERE dream_tracker_id = $1
		ORDER BY deadline_date ASC NULLS LAST, created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, dreamTrackerID)
	if err != nil {
		return nil, fmt.Errorf("find dream key milestones: %w", err)
	}
	defer rows.Close()

	items := make([]model.DreamKeyMilestone, 0)
	for rows.Next() {
		var item model.DreamKeyMilestone
		var description sql.NullString
		var deadlineDate sql.NullTime
		if err := rows.Scan(
			&item.DreamMilestoneID,
			&item.DreamTrackerID,
			&item.Title,
			&description,
			&deadlineDate,
			&item.IsRequired,
			&item.Status,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dream key milestone: %w", err)
		}
		assignNullString(&item.Description, description)
		assignNullTime(&item.DeadlineDate, deadlineDate)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dream key milestones: %w", err)
	}
	return items, nil
}

func (r *DBDreamTrackerRepository) findDreamTrackerFundings(ctx context.Context, tracker model.DreamTracker) ([]model.DreamTrackerFundingOption, error) {
	if tracker.AdmissionID == nil && tracker.FundingID == nil {
		return []model.DreamTrackerFundingOption{}, nil
	}

	if tracker.AdmissionID == nil {
		return r.findSelectedFundingOnly(ctx, tracker)
	}

	query := `
		SELECT fo.funding_id, fo.nama_beasiswa, fo.deskripsi, fo.provider, fo.tipe_pembiayaan, fo.website
		FROM admission_funding af
		INNER JOIN funding_options fo ON fo.funding_id = af.funding_id
		WHERE af.admission_id = $1
		ORDER BY fo.nama_beasiswa ASC
	`
	rows, err := r.db.QueryContext(ctx, query, *tracker.AdmissionID)
	if err != nil {
		return nil, fmt.Errorf("find dream tracker fundings: %w", err)
	}
	defer rows.Close()

	items := make([]model.DreamTrackerFundingOption, 0)
	for rows.Next() {
		item, err := scanDreamTrackerFunding(rows)
		if err != nil {
			return nil, err
		}
		item.Status = fundingStatusForTracker(item.FundingID, tracker.FundingID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dream tracker fundings: %w", err)
	}
	items, err = r.appendMissingSelectedFunding(ctx, items, tracker.FundingID)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *DBDreamTrackerRepository) findSelectedFundingOnly(ctx context.Context, tracker model.DreamTracker) ([]model.DreamTrackerFundingOption, error) {
	if tracker.FundingID == nil {
		return []model.DreamTrackerFundingOption{}, nil
	}

	selectedFunding, err := r.findFundingByID(ctx, *tracker.FundingID)
	if err != nil {
		return nil, err
	}
	selectedFunding.Status = model.DreamTrackerFundingStatusSelected
	return []model.DreamTrackerFundingOption{selectedFunding}, nil
}

func (r *DBDreamTrackerRepository) appendMissingSelectedFunding(
	ctx context.Context,
	items []model.DreamTrackerFundingOption,
	fundingID *string,
) ([]model.DreamTrackerFundingOption, error) {
	if len(items) > 0 || fundingID == nil {
		return items, nil
	}

	selectedFunding, err := r.findFundingByID(ctx, *fundingID)
	if err != nil {
		return nil, err
	}
	selectedFunding.Status = model.DreamTrackerFundingStatusSelected
	return append(items, selectedFunding), nil
}

func fundingStatusForTracker(fundingID string, selectedFundingID *string) model.DreamTrackerFundingStatus {
	if selectedFundingID != nil && fundingID == *selectedFundingID {
		return model.DreamTrackerFundingStatusSelected
	}
	return model.DreamTrackerFundingStatusAvailable
}

func (r *DBDreamTrackerRepository) findFundingByID(ctx context.Context, fundingID string) (model.DreamTrackerFundingOption, error) {
	query := `
		SELECT funding_id, nama_beasiswa, deskripsi, provider, tipe_pembiayaan, website
		FROM funding_options
		WHERE funding_id = $1
	`
	row, err := r.db.QueryContext(ctx, query, fundingID)
	if err != nil {
		return model.DreamTrackerFundingOption{}, fmt.Errorf("find funding by id: %w", err)
	}
	defer row.Close()
	if !row.Next() {
		if err := row.Err(); err != nil {
			return model.DreamTrackerFundingOption{}, fmt.Errorf("find funding by id: %w", err)
		}
		return model.DreamTrackerFundingOption{}, nil
	}
	return scanDreamTrackerFunding(row)
}

func scanDreamTrackerFunding(scanner interface {
	Scan(dest ...any) error
}) (model.DreamTrackerFundingOption, error) {
	var item model.DreamTrackerFundingOption
	var description sql.NullString
	if err := scanner.Scan(
		&item.FundingID,
		&item.NamaBeasiswa,
		&description,
		&item.Provider,
		&item.TipePembiayaan,
		&item.Website,
	); err != nil {
		return model.DreamTrackerFundingOption{}, fmt.Errorf("scan dream tracker funding: %w", err)
	}
	assignNullString(&item.Deskripsi, description)
	return item, nil
}

func assignNullTime(target **time.Time, value sql.NullTime) {
	if value.Valid {
		timeValue := value.Time
		*target = &timeValue
	}
}

func buildRequirementAction(status model.DreamRequirementStatusValue) (string, bool, bool) {
	switch status {
	case model.DreamRequirementStatusRejected:
		return "Upload Ulang", true, true
	case model.DreamRequirementStatusNotUploaded:
		return "Upload", true, false
	default:
		return "", false, false
	}
}

func assignNullString(target **string, value sql.NullString) {
	if value.Valid {
		*target = &value.String
	}
}
