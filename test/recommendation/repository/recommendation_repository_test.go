package repository_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
)

type fakeRecResult struct {
	affected int64
	err      error
}

func (r fakeRecResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeRecResult) RowsAffected() (int64, error) { return r.affected, r.err }

type fakeRecRows struct {
	columns []string
	rows    [][]driver.Value
	idx     int
}

func (r *fakeRecRows) Columns() []string { return r.columns }
func (r *fakeRecRows) Close() error      { return nil }
func (r *fakeRecRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}

type fakeRecDBBehavior struct {
	execFn  func(query string, args []driver.NamedValue) (driver.Result, error)
	queryFn func(query string, args []driver.NamedValue) (driver.Rows, error)
}

type fakeRecDriver struct {
	behavior *fakeRecDBBehavior
}

func (d *fakeRecDriver) Open(name string) (driver.Conn, error) {
	return &fakeRecConn{behavior: d.behavior}, nil
}

type fakeRecConn struct {
	behavior *fakeRecDBBehavior
}

func (c *fakeRecConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *fakeRecConn) Close() error { return nil }

func (c *fakeRecConn) Begin() (driver.Tx, error) {
	return &fakeRecTx{}, nil
}

func (c *fakeRecConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return &fakeRecTx{}, nil
}

func (c *fakeRecConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.behavior.execFn == nil {
		return fakeRecResult{affected: 1}, nil
	}
	return c.behavior.execFn(query, args)
}

func (c *fakeRecConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.behavior.queryFn == nil {
		return &fakeRecRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
	}
	return c.behavior.queryFn(query, args)
}

type fakeRecTx struct{}

func (t *fakeRecTx) Commit() error   { return nil }
func (t *fakeRecTx) Rollback() error { return nil }

var fakeRecDriverSeq int

func newFakeRecDB(t *testing.T, behavior *fakeRecDBBehavior) *sql.DB {
	t.Helper()
	fakeRecDriverSeq++
	driverName := fmt.Sprintf("fakedb_recommendation_%d", fakeRecDriverSeq)
	sql.Register(driverName, &fakeRecDriver{behavior: behavior})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open fake recommendation db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRecommendationRepositoryImplementsContract(t *testing.T) {
	var _ repository.RecommendationRepository = (*repository.DBRecommendationRepository)(nil)
}

func TestCreateSubmissionRepository(t *testing.T) {
	execCount := 0
	db := newFakeRecDB(t, &fakeRecDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			execCount++
			return fakeRecResult{affected: 1}, nil
		},
	})
	repo := repository.NewRecommendationRepository(db)
	now := time.Now().UTC()
	tid := "doc-transcript"
	cid := "doc-cv"

	submission, err := repo.CreateSubmission(context.Background(), repository.CreateSubmissionParams{
		Submission: model.RecommendationSubmission{
			RecSubmissionID:      "submission-1",
			UserID:               "user-1",
			TranscriptDocumentID: &tid,
			CVDocumentID:         &cid,
			Status:               model.RecommendationStatusDraft,
			CreatedAt:            now,
		},
		Preferences: []model.RecommendationPreference{{
			PrefID:          "pref-1",
			RecSubmissionID: "submission-1",
			PreferenceKey:   "country",
			PreferenceValue: "Japan",
			CreatedAt:       now,
		}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if submission.RecSubmissionID != "submission-1" {
		t.Fatalf("expected submission id submission-1, got %s", submission.RecSubmissionID)
	}
	if execCount != 2 {
		t.Fatalf("expected 2 exec calls (submission + pref), got %d", execCount)
	}
}

func TestCreateDocumentRepository(t *testing.T) {
	db := newFakeRecDB(t, &fakeRecDBBehavior{})
	repo := repository.NewRecommendationRepository(db)
	now := time.Now().UTC()

	doc, err := repo.CreateDocument(context.Background(), model.Document{
		DocumentID:       "doc-1",
		UserID:           "user-1",
		OriginalFilename: "cv.pdf",
		StoragePath:      "uploads/user-1/cv.pdf",
		PublicURL:        "uploads/user-1/cv.pdf",
		MIMEType:         "application/pdf",
		SizeBytes:        99,
		DocumentType:     model.DocumentTypeCV,
		UploadedAt:       now,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if doc.DocumentID != "doc-1" {
		t.Fatalf("expected document id doc-1, got %s", doc.DocumentID)
	}
}

func TestUpdateSubmissionStatusNotFoundRepository(t *testing.T) {
	db := newFakeRecDB(t, &fakeRecDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			if strings.Contains(query, "UPDATE recommendation_submissions") {
				return fakeRecResult{affected: 0}, nil
			}
			return fakeRecResult{affected: 1}, nil
		},
	})
	repo := repository.NewRecommendationRepository(db)

	err := repo.UpdateSubmissionStatus(context.Background(), "submission-1", "user-1", model.RecommendationStatusCompleted)
	if err != errs.ErrSubmissionNotFound {
		t.Fatalf("expected %v, got %v", errs.ErrSubmissionNotFound, err)
	}
}

func TestCreateResultSetEmptyRowsRepository(t *testing.T) {
	repo := repository.NewRecommendationRepository(nil)
	_, err := repo.CreateResultSet(context.Background(), "submission-1", time.Now().UTC(), nil)
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestFindSubmissionDetailNotFoundRepository(t *testing.T) {
	db := newFakeRecDB(t, &fakeRecDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if strings.Contains(query, "FROM recommendation_submissions") {
				return &fakeRecRows{
					columns: []string{"rec_submission_id", "user_id", "transcript_document_id", "cv_document_id", "status", "created_at", "submitted_at"},
					rows:    [][]driver.Value{},
				}, nil
			}
			return &fakeRecRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
		},
	})
	repo := repository.NewRecommendationRepository(db)

	_, err := repo.FindSubmissionDetail(context.Background(), "submission-x", "user-1")
	if err != errs.ErrSubmissionNotFound {
		t.Fatalf("expected %v, got %v", errs.ErrSubmissionNotFound, err)
	}
}

func TestFindDocumentByIDAndUserRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeRecDB(t, &fakeRecDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if strings.Contains(query, "FROM documents") {
				return &fakeRecRows{
					columns: []string{"document_id", "user_id", "original_filename", "storage_path", "public_url", "mime_type", "size_bytes", "document_type", "uploaded_at"},
					rows: [][]driver.Value{
						{"doc-1", "user-1", "transcript.pdf", "/tmp/doc-1.pdf", "http://local/doc-1.pdf", "application/pdf", int64(100), string(model.DocumentTypeTranscript), now},
					},
				}, nil
			}
			return &fakeRecRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
		},
	})
	repo := repository.NewRecommendationRepository(db)

	doc, err := repo.FindDocumentByIDAndUser(context.Background(), "doc-1", "user-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if doc.DocumentID != "doc-1" {
		t.Fatalf("expected doc-1, got %s", doc.DocumentID)
	}
}

func TestCreateResultSetRepositorySuccess(t *testing.T) {
	execCount := 0
	db := newFakeRecDB(t, &fakeRecDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			execCount++
			return fakeRecResult{affected: 1}, nil
		},
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if strings.Contains(query, "COALESCE(MAX(version_no), 0)") {
				return &fakeRecRows{
					columns: []string{"coalesce"},
					rows:    [][]driver.Value{{int64(2)}},
				}, nil
			}
			return &fakeRecRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
		},
	})
	repo := repository.NewRecommendationRepository(db)
	now := time.Now().UTC()

	set, err := repo.CreateResultSet(context.Background(), "submission-1", now, []model.RecommendationResult{{
		RecResultID:       "res-1",
		ResultSetID:       "set-1",
		RankNo:            1,
		UniversityName:    "University A",
		ProgramName:       "CS",
		Country:           "Japan",
		FitScore:          90,
		FitLevel:          "high",
		Overview:          "overview",
		WhyThisUniversity: "why",
		WhyThisProgram:    "why program",
		ReasonSummary:     "summary",
		ProsJSON:          `["pro"]`,
		ConsJSON:          `["con"]`,
		CreatedAt:         now,
	}})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if set.VersionNo != 3 {
		t.Fatalf("expected version 3, got %d", set.VersionNo)
	}
	if execCount != 2 {
		t.Fatalf("expected 2 execs, got %d", execCount)
	}
}

func TestFindSubmissionDetailRepositorySuccess(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeRecDB(t, &fakeRecDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			switch {
			case strings.Contains(query, "FROM recommendation_submissions"):
				return &fakeRecRows{
					columns: []string{"rec_submission_id", "user_id", "transcript_document_id", "cv_document_id", "status", "created_at", "submitted_at"},
					rows: [][]driver.Value{
						{"submission-1", "user-1", "doc-transcript", "doc-cv", string(model.RecommendationStatusCompleted), now, now},
					},
				}, nil
			case strings.Contains(query, "FROM documents"):
				docID := args[0].Value.(string)
				return &fakeRecRows{
					columns: []string{"document_id", "user_id", "original_filename", "storage_path", "public_url", "mime_type", "size_bytes", "document_type", "uploaded_at"},
					rows: [][]driver.Value{
						{docID, "user-1", docID + ".pdf", "/tmp/" + docID + ".pdf", "http://local/" + docID + ".pdf", "application/pdf", int64(100), string(model.DocumentTypeTranscript), now},
					},
				}, nil
			case strings.Contains(query, "FROM recommendation_preferences"):
				return &fakeRecRows{
					columns: []string{"pref_id", "rec_submission_id", "pref_key", "pref_value", "created_at"},
					rows: [][]driver.Value{
						{"pref-1", "submission-1", "countries", "Japan", now},
					},
				}, nil
			case strings.Contains(query, "FROM recommendation_result_sets"):
				return &fakeRecRows{
					columns: []string{"result_set_id", "rec_submission_id", "version_no", "generated_at", "created_at"},
					rows: [][]driver.Value{
						{"set-1", "submission-1", int64(1), now, now},
					},
				}, nil
			case strings.Contains(query, "FROM recommendation_results"):
				return &fakeRecRows{
					columns: []string{"rec_result_id", "result_set_id", "program_id", "admission_id", "rank_no", "university_name", "program_name", "country", "fit_score", "score", "fit_level", "overview", "why_this_university", "why_this_program", "reason_summary", "pros_json", "cons_json", "created_at"},
					rows: [][]driver.Value{
						{"res-1", "set-1", "program-1", "admission-1", int64(1), "University A", "CS", "Japan", int64(90), int64(88), "high", "overview", "why uni", "why program", "summary", `["pro"]`, `["con"]`, now},
					},
				}, nil
			default:
				return &fakeRecRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
			}
		},
	})
	repo := repository.NewRecommendationRepository(db)

	detail, err := repo.FindSubmissionDetail(context.Background(), "submission-1", "user-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if detail.Submission.RecSubmissionID != "submission-1" {
		t.Fatalf("unexpected submission id %s", detail.Submission.RecSubmissionID)
	}
	if len(detail.Documents) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(detail.Documents))
	}
	if len(detail.Preferences) != 1 {
		t.Fatalf("expected 1 preference, got %d", len(detail.Preferences))
	}
	if detail.LatestResultSet == nil || len(detail.Results) != 1 {
		t.Fatalf("expected result set and result, got %#v %#v", detail.LatestResultSet, detail.Results)
	}
}
