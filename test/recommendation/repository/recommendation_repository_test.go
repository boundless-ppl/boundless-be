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
