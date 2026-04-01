package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"boundless-be/errs"
	"boundless-be/model"
)

type CreateSubmissionParams struct {
	Submission  model.RecommendationSubmission
	Preferences []model.RecommendationPreference
}

type SubmissionDetail struct {
	Submission      model.RecommendationSubmission
	Documents       []model.Document
	Preferences     []model.RecommendationPreference
	LatestResultSet *model.RecommendationResultSet
	Results         []model.RecommendationResult
}

type RecommendationRepository interface {
	CreateDocument(ctx context.Context, doc model.Document) (model.Document, error)
	FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error)
	CreateSubmission(ctx context.Context, params CreateSubmissionParams) (model.RecommendationSubmission, error)
	UpdateSubmissionStatus(ctx context.Context, submissionID, userID string, status model.RecommendationStatus) error
	CreateResultSet(ctx context.Context, submissionID string, generatedAt time.Time, results []model.RecommendationResult) (model.RecommendationResultSet, error)
	FindSubmissionDetail(ctx context.Context, submissionID, userID string) (SubmissionDetail, error)
}

type DBRecommendationRepository struct {
	db *sql.DB
}

func NewRecommendationRepository(db *sql.DB) *DBRecommendationRepository {
	return &DBRecommendationRepository{db: db}
}

func (r *DBRecommendationRepository) CreateDocument(ctx context.Context, doc model.Document) (model.Document, error) {
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

func (r *DBRecommendationRepository) FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error) {
	query := `
		SELECT document_id, user_id, original_filename, storage_path, public_url, mime_type, size_bytes, document_type, uploaded_at
		FROM documents
		WHERE document_id = $1 AND user_id = $2
	`
	var doc model.Document
	err := r.db.QueryRowContext(ctx, query, documentID, userID).Scan(
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
			return model.Document{}, errs.ErrDocumentNotFound
		}
		return model.Document{}, fmt.Errorf("find document by id and user: %w", err)
	}
	return doc, nil
}

func (r *DBRecommendationRepository) CreateSubmission(ctx context.Context, params CreateSubmissionParams) (model.RecommendationSubmission, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.RecommendationSubmission{}, fmt.Errorf("begin tx create submission: %w", err)
	}
	defer tx.Rollback()

	createSubmissionQuery := `
		INSERT INTO recommendation_submissions
		(rec_submission_id, user_id, transcript_document_id, cv_document_id, status, created_at, submitted_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`

	_, err = tx.ExecContext(
		ctx,
		createSubmissionQuery,
		params.Submission.RecSubmissionID,
		params.Submission.UserID,
		nullString(params.Submission.TranscriptDocumentID),
		nullString(params.Submission.CVDocumentID),
		params.Submission.Status,
		params.Submission.CreatedAt,
		nullTimePtr(params.Submission.SubmittedAt),
	)
	if err != nil {
		return model.RecommendationSubmission{}, fmt.Errorf("insert recommendation submission: %w", err)
	}

	prefQuery := `
		INSERT INTO recommendation_preferences
		(pref_id, rec_submission_id, pref_key, pref_value, created_at)
		VALUES ($1,$2,$3,$4,$5)
	`
	for _, pref := range params.Preferences {
		_, err := tx.ExecContext(
			ctx,
			prefQuery,
			pref.PrefID,
			pref.RecSubmissionID,
			pref.PreferenceKey,
			pref.PreferenceValue,
			pref.CreatedAt,
		)
		if err != nil {
			return model.RecommendationSubmission{}, fmt.Errorf("insert recommendation preference: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return model.RecommendationSubmission{}, fmt.Errorf("commit create submission: %w", err)
	}

	return params.Submission, nil
}

func (r *DBRecommendationRepository) UpdateSubmissionStatus(ctx context.Context, submissionID, userID string, status model.RecommendationStatus) error {
	query := `
		UPDATE recommendation_submissions
		SET status = $3
		WHERE rec_submission_id = $1 AND user_id = $2
	`
	result, err := r.db.ExecContext(ctx, query, submissionID, userID, status)
	if err != nil {
		return fmt.Errorf("update recommendation submission status: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows update status: %w", err)
	}
	if affected == 0 {
		return errs.ErrSubmissionNotFound
	}

	return nil
}

func (r *DBRecommendationRepository) CreateResultSet(ctx context.Context, submissionID string, generatedAt time.Time, results []model.RecommendationResult) (model.RecommendationResultSet, error) {
	if len(results) == 0 {
		return model.RecommendationResultSet{}, errs.ErrInvalidInput
	}

	setID := results[0].ResultSetID
	if setID == "" {
		return model.RecommendationResultSet{}, errs.ErrInvalidInput
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.RecommendationResultSet{}, fmt.Errorf("begin tx create result set: %w", err)
	}
	defer tx.Rollback()

	versionQuery := `
		SELECT COALESCE(MAX(version_no), 0)
		FROM recommendation_result_sets
		WHERE rec_submission_id = $1
	`
	var maxVersion int
	if err := tx.QueryRowContext(ctx, versionQuery, submissionID).Scan(&maxVersion); err != nil {
		return model.RecommendationResultSet{}, fmt.Errorf("get max result set version: %w", err)
	}

	set := model.RecommendationResultSet{
		ResultSetID:     results[0].ResultSetID,
		RecSubmissionID: submissionID,
		VersionNo:       maxVersion + 1,
		GeneratedAt:     generatedAt,
		CreatedAt:       time.Now().UTC(),
	}

	insertSetQuery := `
		INSERT INTO recommendation_result_sets
		(result_set_id, rec_submission_id, version_no, generated_at, created_at)
		VALUES ($1,$2,$3,$4,$5)
	`
	_, err = tx.ExecContext(ctx, insertSetQuery, set.ResultSetID, set.RecSubmissionID, set.VersionNo, set.GeneratedAt, set.CreatedAt)
	if err != nil {
		return model.RecommendationResultSet{}, fmt.Errorf("insert recommendation result set: %w", err)
	}

	insertResultQuery := `
		INSERT INTO recommendation_results
		(rec_result_id, result_set_id, rank_no, university_name, program_name, country, fit_score, fit_level, overview,
		 why_this_university, why_this_program, reason_summary, pros_json, cons_json, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`
	for _, row := range results {
		_, err := tx.ExecContext(
			ctx,
			insertResultQuery,
			row.RecResultID,
			row.ResultSetID,
			row.RankNo,
			row.UniversityName,
			row.ProgramName,
			row.Country,
			row.FitScore,
			row.FitLevel,
			row.Overview,
			row.WhyThisUniversity,
			row.WhyThisProgram,
			row.ReasonSummary,
			row.ProsJSON,
			row.ConsJSON,
			row.CreatedAt,
		)
		if err != nil {
			return model.RecommendationResultSet{}, fmt.Errorf("insert recommendation result: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return model.RecommendationResultSet{}, fmt.Errorf("commit create result set: %w", err)
	}

	return set, nil
}

func (r *DBRecommendationRepository) FindSubmissionDetail(ctx context.Context, submissionID, userID string) (SubmissionDetail, error) {
	detail, transcriptID, cvID, err := r.findSubmission(ctx, submissionID, userID)
	if err != nil {
		return SubmissionDetail{}, err
	}

	if err := r.attachSubmissionDocuments(ctx, &detail, transcriptID, cvID); err != nil {
		return SubmissionDetail{}, err
	}

	preferences, err := r.findSubmissionPreferences(ctx, submissionID)
	if err != nil {
		return SubmissionDetail{}, err
	}
	detail.Preferences = preferences

	latestSet, err := r.findLatestResultSet(ctx, submissionID)
	if err != nil {
		return SubmissionDetail{}, err
	}
	if latestSet == nil {
		return detail, nil
	}
	detail.LatestResultSet = latestSet

	results, err := r.findResultsByResultSetID(ctx, latestSet.ResultSetID)
	if err != nil {
		return SubmissionDetail{}, err
	}
	detail.Results = results

	return detail, nil
}

func (r *DBRecommendationRepository) findDocumentByID(ctx context.Context, documentID string) (model.Document, error) {
	query := `
		SELECT document_id, user_id, original_filename, storage_path, public_url, mime_type, size_bytes, document_type, uploaded_at
		FROM documents
		WHERE document_id = $1
	`
	var doc model.Document
	err := r.db.QueryRowContext(ctx, query, documentID).Scan(
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
			return model.Document{}, errs.ErrDocumentNotFound
		}
		return model.Document{}, fmt.Errorf("find document by id: %w", err)
	}
	return doc, nil
}

func nullString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func nullTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func (r *DBRecommendationRepository) findSubmission(ctx context.Context, submissionID, userID string) (SubmissionDetail, sql.NullString, sql.NullString, error) {
	submissionQuery := `
		SELECT rec_submission_id, user_id, transcript_document_id, cv_document_id, status, created_at, submitted_at
		FROM recommendation_submissions
		WHERE rec_submission_id = $1 AND user_id = $2
	`

	var detail SubmissionDetail
	var transcriptID sql.NullString
	var cvID sql.NullString
	var submittedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, submissionQuery, submissionID, userID).Scan(
		&detail.Submission.RecSubmissionID,
		&detail.Submission.UserID,
		&transcriptID,
		&cvID,
		&detail.Submission.Status,
		&detail.Submission.CreatedAt,
		&submittedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SubmissionDetail{}, sql.NullString{}, sql.NullString{}, errs.ErrSubmissionNotFound
		}
		return SubmissionDetail{}, sql.NullString{}, sql.NullString{}, fmt.Errorf("find recommendation submission detail: %w", err)
	}
	if submittedAt.Valid {
		detail.Submission.SubmittedAt = &submittedAt.Time
	}
	return detail, transcriptID, cvID, nil
}

func (r *DBRecommendationRepository) attachSubmissionDocuments(ctx context.Context, detail *SubmissionDetail, transcriptID, cvID sql.NullString) error {
	if transcriptID.Valid {
		detail.Submission.TranscriptDocumentID = &transcriptID.String
		doc, err := r.findDocumentByID(ctx, transcriptID.String)
		if err != nil {
			return err
		}
		detail.Documents = append(detail.Documents, doc)
	}
	if cvID.Valid {
		detail.Submission.CVDocumentID = &cvID.String
		doc, err := r.findDocumentByID(ctx, cvID.String)
		if err != nil {
			return err
		}
		detail.Documents = append(detail.Documents, doc)
	}
	return nil
}

func (r *DBRecommendationRepository) findSubmissionPreferences(ctx context.Context, submissionID string) ([]model.RecommendationPreference, error) {
	prefsQuery := `
		SELECT pref_id, rec_submission_id, pref_key, pref_value, created_at
		FROM recommendation_preferences
		WHERE rec_submission_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, prefsQuery, submissionID)
	if err != nil {
		return nil, fmt.Errorf("find recommendation preferences: %w", err)
	}
	defer rows.Close()

	preferences := make([]model.RecommendationPreference, 0)
	for rows.Next() {
		var pref model.RecommendationPreference
		if err := rows.Scan(
			&pref.PrefID,
			&pref.RecSubmissionID,
			&pref.PreferenceKey,
			&pref.PreferenceValue,
			&pref.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan recommendation preference: %w", err)
		}
		preferences = append(preferences, pref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recommendation preferences: %w", err)
	}
	return preferences, nil
}

func (r *DBRecommendationRepository) findLatestResultSet(ctx context.Context, submissionID string) (*model.RecommendationResultSet, error) {
	latestSetQuery := `
		SELECT result_set_id, rec_submission_id, version_no, generated_at, created_at
		FROM recommendation_result_sets
		WHERE rec_submission_id = $1
		ORDER BY version_no DESC
		LIMIT 1
	`
	latestSet := model.RecommendationResultSet{}
	err := r.db.QueryRowContext(ctx, latestSetQuery, submissionID).Scan(
		&latestSet.ResultSetID,
		&latestSet.RecSubmissionID,
		&latestSet.VersionNo,
		&latestSet.GeneratedAt,
		&latestSet.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find latest recommendation result set: %w", err)
	}
	return &latestSet, nil
}

func (r *DBRecommendationRepository) findResultsByResultSetID(ctx context.Context, resultSetID string) ([]model.RecommendationResult, error) {
	resultsQuery := `
		SELECT rec_result_id, result_set_id, rank_no, university_name, program_name, country, fit_score, fit_level,
		       overview, why_this_university, why_this_program, reason_summary, pros_json, cons_json, created_at
		FROM recommendation_results
		WHERE result_set_id = $1
		ORDER BY rank_no ASC
	`
	rows, err := r.db.QueryContext(ctx, resultsQuery, resultSetID)
	if err != nil {
		return nil, fmt.Errorf("find recommendation results: %w", err)
	}
	defer rows.Close()

	results := make([]model.RecommendationResult, 0)
	for rows.Next() {
		var row model.RecommendationResult
		if err := rows.Scan(
			&row.RecResultID,
			&row.ResultSetID,
			&row.RankNo,
			&row.UniversityName,
			&row.ProgramName,
			&row.Country,
			&row.FitScore,
			&row.FitLevel,
			&row.Overview,
			&row.WhyThisUniversity,
			&row.WhyThisProgram,
			&row.ReasonSummary,
			&row.ProsJSON,
			&row.ConsJSON,
			&row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan recommendation result: %w", err)
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recommendation results: %w", err)
	}
	return results, nil
}
