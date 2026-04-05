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
	requirements, err := r.findDreamRequirementDetails(ctx, dreamTrackerID)
	if err != nil {
		return DreamTrackerDetail{}, err
	}
	programInfo, err := r.findDreamTrackerProgramInfo(ctx, tracker)
	if err != nil {
		return DreamTrackerDetail{}, err
	}
	milestones, err := r.findDreamKeyMilestones(ctx, dreamTrackerID)
	if err != nil {
		return DreamTrackerDetail{}, err
	}
	fundings, err := r.findDreamTrackerFundings(ctx, tracker)
	if err != nil {
		return DreamTrackerDetail{}, err
	}
	detail.DreamTracker = tracker
	detail.ProgramInfo = programInfo
	detail.Requirements = requirements
	detail.Milestones = milestones
	detail.Fundings = fundings
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

func (r *DBDreamTrackerRepository) findDreamRequirementDetails(ctx context.Context, dreamTrackerID string) ([]model.DreamRequirementDetail, error) {
	reqQuery := `
		SELECT drs.dream_req_status_id, drs.dream_tracker_id, drs.document_id, drs.req_catalog_id, drs.status, drs.notes, drs.ai_status, drs.ai_messages, drs.created_at,
		       rc.key, rc.label, rc.kategori, rc.deskripsi
		FROM dream_requirement_status drs
		INNER JOIN requirement_catalog rc ON rc.req_catalog_id = drs.req_catalog_id
		WHERE dream_tracker_id = $1
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
	); err != nil {
		return model.DreamRequirementDetail{}, fmt.Errorf("scan dream requirement detail: %w", err)
	}

	assignNullString(&item.DreamRequirementStatus.DocumentID, documentID)
	assignNullString(&item.DreamRequirementStatus.Notes, notes)
	assignNullString(&item.DreamRequirementStatus.AIStatus, aiStatus)
	assignNullString(&item.DreamRequirementStatus.AIMessages, aiMessages)
	assignNullString(&item.RequirementDescription, requirementDescription)
	item.ActionLabel, item.CanUpload, item.NeedsReupload = buildRequirementAction(item.DreamRequirementStatus.Status)

	return item, nil
}

func (r *DBDreamTrackerRepository) findDreamTrackerProgramInfo(ctx context.Context, tracker model.DreamTracker) (model.DreamTrackerProgramInfo, error) {
	info := model.DreamTrackerProgramInfo{ProgramID: tracker.ProgramID}

	query := `
		SELECT ap.nama, ap.intake, ap.deadline, ap.website_url, rr.program_name, rr.university_name
		FROM dream_tracker dt
		LEFT JOIN admission_paths ap ON ap.admission_id = dt.admission_id
		LEFT JOIN recommendation_results rr ON rr.rec_result_id = dt.source_rec_result_id
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
