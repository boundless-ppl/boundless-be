package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

type RecommendationCandidate struct {
	ProgramID             string
	ProgramName           string
	UniversityName        string
	Country               string
	DegreeLevel           string
	Language              string
	FundingSummary        string
	AdmissionDeadline     string
	OfficialProgramURL    string
	OfficialUniversityURL string
}

type RecommendationRepository interface {
	CreateDocument(ctx context.Context, doc model.Document) (model.Document, error)
	FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error)
	CreateSubmission(ctx context.Context, params CreateSubmissionParams) (model.RecommendationSubmission, error)
	UpdateSubmissionStatus(ctx context.Context, submissionID, userID string, status model.RecommendationStatus) error
	CreateResultSet(ctx context.Context, submissionID string, generatedAt time.Time, results []model.RecommendationResult) (model.RecommendationResultSet, error)
	FindSubmissionDetail(ctx context.Context, submissionID, userID string) (SubmissionDetail, error)
	ListRecommendationCandidates(ctx context.Context, preferences []model.RecommendationPreference) ([]RecommendationCandidate, error)
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
		(rec_result_id, result_set_id, program_id, rank_no, university_name, program_name, country, fit_score, fit_level, overview,
		 why_this_university, why_this_program, reason_summary, pros_json, cons_json, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`
	for _, row := range results {
		_, err := tx.ExecContext(
			ctx,
			insertResultQuery,
			row.RecResultID,
			row.ResultSetID,
			nullEmptyString(row.ProgramID),
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
		SELECT rec_result_id, result_set_id, program_id, rank_no, university_name, program_name, country, fit_score, fit_level,
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
			&row.ProgramID,
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

func (r *DBRecommendationRepository) ListRecommendationCandidates(ctx context.Context, preferences []model.RecommendationPreference) ([]RecommendationCandidate, error) {
	continents := make([]string, 0)
	countries := make([]string, 0)
	fields := make([]string, 0)
	languages := make([]string, 0)
	startPeriods := make([]string, 0)
	scholarshipTypes := make([]string, 0)
	degreeLevel := ""
	for _, pref := range preferences {
		switch strings.ToLower(strings.TrimSpace(pref.PreferenceKey)) {
		case "continents", "continent":
			if value := strings.TrimSpace(pref.PreferenceValue); value != "" {
				continents = append(continents, value)
			}
		case "countries", "country":
			if value := strings.TrimSpace(pref.PreferenceValue); value != "" {
				countries = append(countries, value)
			}
		case "fields_of_study", "field_of_study", "field":
			if value := strings.TrimSpace(pref.PreferenceValue); value != "" {
				fields = append(fields, value)
			}
		case "degree_level":
			if value := strings.TrimSpace(pref.PreferenceValue); value != "" && degreeLevel == "" {
				degreeLevel = normalizeRecommendationDegreeLevel(value)
			}
		case "languages", "language":
			if value := strings.TrimSpace(pref.PreferenceValue); value != "" {
				languages = append(languages, value)
			}
		case "start_periods", "start_period":
			if value := strings.TrimSpace(pref.PreferenceValue); value != "" {
				startPeriods = append(startPeriods, value)
			}
		case "scholarship_types", "scholarship_type":
			if value := strings.TrimSpace(pref.PreferenceValue); value != "" {
				scholarshipTypes = append(scholarshipTypes, value)
			}
		}
	}

	query := `
		SELECT
			p.program_id,
			p.nama AS program_name,
			u.nama AS university_name,
			COALESCE(c.nama, u.negara_id) AS country,
			p.jenjang AS degree_level,
			p.bahasa AS language,
			COALESCE(
				string_agg(DISTINCT fo.nama_beasiswa, ' || ' ORDER BY fo.nama_beasiswa)
				FILTER (WHERE COALESCE(fo.nama_beasiswa, '') <> ''),
				''
			) AS funding_summary,
			COALESCE(TO_CHAR(MIN(ap.deadline), 'YYYY-MM-DD'), '') AS admission_deadline,
			COALESCE(p.program_url, '') AS official_program_url,
			COALESCE(u.website, '') AS official_university_url
		FROM programs p
		JOIN universities u ON u.id = p.university_id
		LEFT JOIN countries c ON c.negara_id = u.negara_id
		LEFT JOIN admission_paths ap ON ap.program_id = p.program_id
		LEFT JOIN admission_funding af ON af.admission_id = ap.admission_id
		LEFT JOIN funding_options fo ON fo.funding_id = af.funding_id
		WHERE 1=1
	`
	args := make([]any, 0)
	index := 1
	if len(continents) > 0 {
		query += ` AND (`
		for i, continent := range continents {
			if i > 0 {
				query += ` OR `
			}
			query += `COALESCE(c.benua, '') ILIKE $` + fmt.Sprint(index)
			args = append(args, continent)
			index++
		}
		query += `)`
	}
	if len(countries) > 0 {
		query += ` AND (`
		for i, country := range countries {
			if i > 0 {
				query += ` OR `
			}
			query += `(COALESCE(c.nama, '') ILIKE $` + fmt.Sprint(index) + ` OR COALESCE(u.negara_id, '') ILIKE $` + fmt.Sprint(index) + `)`
			args = append(args, country)
			index++
		}
		query += `)`
	}
	if len(fields) > 0 {
		query += ` AND (` 
		for i, field := range fields {
			if i > 0 {
				query += ` OR `
			}
			query += `p.nama ILIKE $` + fmt.Sprint(index)
			args = append(args, "%"+field+"%")
			index++
		}
		query += `)`
	}
	if degreeLevel != "" {
		query += ` AND p.jenjang ILIKE $` + fmt.Sprint(index)
		args = append(args, degreeLevel)
		index++
	}
	if len(languages) > 0 {
		query += ` AND (`
		for i, language := range languages {
			if i > 0 {
				query += ` OR `
			}
			query += `p.bahasa ILIKE $` + fmt.Sprint(index)
			args = append(args, "%"+language+"%")
			index++
		}
		query += `)`
	}
	if len(startPeriods) > 0 {
		query += ` AND (`
		for i, period := range startPeriods {
			if i > 0 {
				query += ` OR `
			}
			query += `COALESCE(ap.intake, '') ILIKE $` + fmt.Sprint(index)
			args = append(args, "%"+period+"%")
			index++
		}
		query += `)`
	}
	if len(scholarshipTypes) > 0 {
		query += ` AND (`
		for i, scholarshipType := range scholarshipTypes {
			if i > 0 {
				query += ` OR `
			}
			query += `COALESCE(fo.tipe_pembiayaan, '') ILIKE $` + fmt.Sprint(index)
			args = append(args, scholarshipType)
			index++
		}
		query += `)`
	}
	query += `
		GROUP BY
			p.program_id, p.nama, u.nama, c.nama, p.jenjang, p.bahasa, p.program_url, u.website, u.ranking, u.negara_id
		ORDER BY
			COALESCE(u.ranking, 2147483647) ASC,
			u.nama ASC,
			p.nama ASC
		LIMIT 30
	`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list recommendation candidates: %w", err)
	}
	defer rows.Close()

	result := make([]RecommendationCandidate, 0)
	for rows.Next() {
		var item RecommendationCandidate
		if err := rows.Scan(
			&item.ProgramID,
			&item.ProgramName,
			&item.UniversityName,
			&item.Country,
			&item.DegreeLevel,
			&item.Language,
			&item.FundingSummary,
			&item.AdmissionDeadline,
			&item.OfficialProgramURL,
			&item.OfficialUniversityURL,
		); err != nil {
			return nil, fmt.Errorf("scan recommendation candidate: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recommendation candidates: %w", err)
	}
	return result, nil
}

func normalizeRecommendationDegreeLevel(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "master", "masters", "s2", "postgraduate", "graduate":
		return "S2"
	case "phd", "doctorate", "doctoral", "doctor", "s3":
		return "S3"
	case "bachelor", "bachelors", "undergraduate", "s1":
		return "S1"
	default:
		return value
	}
}

func nullEmptyString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
