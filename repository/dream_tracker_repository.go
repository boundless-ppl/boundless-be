package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"boundless-be/errs"
	"boundless-be/model"

	"github.com/google/uuid"
)

type DreamTrackerDetail struct {
	DreamTracker model.DreamTracker
	Requirements []model.DreamRequirementStatus
}

type DreamTrackerRepository interface {
	CreateDreamTracker(ctx context.Context, tracker model.DreamTracker) (model.DreamTracker, error)
	FindDreamTrackerDetail(ctx context.Context, dreamTrackerID, userID string) (DreamTrackerDetail, error)
	FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error)
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
	requirements, err := r.findDreamRequirementStatuses(ctx, dreamTrackerID)
	if err != nil {
		return DreamTrackerDetail{}, err
	}
	detail.DreamTracker = tracker
	detail.Requirements = requirements
	return detail, nil
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

	var tracker model.DreamTracker
	var admissionID sql.NullString
	var fundingID sql.NullString
	var reqSubmissionID sql.NullString
	var sourceRecResultID sql.NullString

	err := r.db.QueryRowContext(ctx, query, dreamTrackerID, userID).Scan(
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
		if errors.Is(err, sql.ErrNoRows) {
			return model.DreamTracker{}, errs.ErrDreamTrackerNotFound
		}
		return model.DreamTracker{}, fmt.Errorf("find dream tracker detail: %w", err)
	}

	assignNullString(&tracker.AdmissionID, admissionID)
	assignNullString(&tracker.FundingID, fundingID)
	assignNullString(&tracker.ReqSubmissionID, reqSubmissionID)
	assignNullString(&tracker.SourceRecResultID, sourceRecResultID)

	return tracker, nil
}

func (r *DBDreamTrackerRepository) findDreamRequirementStatuses(ctx context.Context, dreamTrackerID string) ([]model.DreamRequirementStatus, error) {
	reqQuery := `
		SELECT dream_req_status_id, dream_tracker_id, document_id, req_catalog_id, status, notes, ai_status, ai_messages, created_at
		FROM dream_requirement_status
		WHERE dream_tracker_id = $1
		ORDER BY created_at ASC, dream_req_status_id ASC
	`
	rows, err := r.db.QueryContext(ctx, reqQuery, dreamTrackerID)
	if err != nil {
		return nil, fmt.Errorf("find dream requirement statuses: %w", err)
	}
	defer rows.Close()

	items := make([]model.DreamRequirementStatus, 0)
	for rows.Next() {
		item, err := scanDreamRequirementStatus(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dream requirement statuses: %w", err)
	}
	return items, nil
}

func scanDreamRequirementStatus(scanner interface {
	Scan(dest ...any) error
}) (model.DreamRequirementStatus, error) {
	var item model.DreamRequirementStatus
	var documentID sql.NullString
	var notes sql.NullString
	var aiStatus sql.NullString
	var aiMessages sql.NullString

	if err := scanner.Scan(
		&item.DreamReqStatusID,
		&item.DreamTrackerID,
		&documentID,
		&item.ReqCatalogID,
		&item.Status,
		&notes,
		&aiStatus,
		&aiMessages,
		&item.CreatedAt,
	); err != nil {
		return model.DreamRequirementStatus{}, fmt.Errorf("scan dream requirement status: %w", err)
	}

	assignNullString(&item.DocumentID, documentID)
	assignNullString(&item.Notes, notes)
	assignNullString(&item.AIStatus, aiStatus)
	assignNullString(&item.AIMessages, aiMessages)

	return item, nil
}

func assignNullString(target **string, value sql.NullString) {
	if value.Valid {
		*target = &value.String
	}
}
