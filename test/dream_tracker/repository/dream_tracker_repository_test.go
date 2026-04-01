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

type fakeDreamResult struct {
	affected int64
	err      error
}

func (r fakeDreamResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeDreamResult) RowsAffected() (int64, error) { return r.affected, r.err }

type fakeDreamRows struct {
	columns []string
	rows    [][]driver.Value
	idx     int
}

func (r *fakeDreamRows) Columns() []string { return r.columns }
func (r *fakeDreamRows) Close() error      { return nil }
func (r *fakeDreamRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}

type fakeDreamDBBehavior struct {
	execFn  func(query string, args []driver.NamedValue) (driver.Result, error)
	queryFn func(query string, args []driver.NamedValue) (driver.Rows, error)
}

type fakeDreamDriver struct {
	behavior *fakeDreamDBBehavior
}

func (d *fakeDreamDriver) Open(name string) (driver.Conn, error) {
	return &fakeDreamConn{behavior: d.behavior}, nil
}

type fakeDreamConn struct {
	behavior *fakeDreamDBBehavior
}

func (c *fakeDreamConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c *fakeDreamConn) Close() error              { return nil }
func (c *fakeDreamConn) Begin() (driver.Tx, error) { return &fakeDreamTx{}, nil }
func (c *fakeDreamConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return &fakeDreamTx{}, nil
}
func (c *fakeDreamConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.behavior.execFn == nil {
		return fakeDreamResult{affected: 1}, nil
	}
	return c.behavior.execFn(query, args)
}
func (c *fakeDreamConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.behavior.queryFn == nil {
		return &fakeDreamRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
	}
	return c.behavior.queryFn(query, args)
}

type fakeDreamTx struct{}

func (t *fakeDreamTx) Commit() error   { return nil }
func (t *fakeDreamTx) Rollback() error { return nil }

var fakeDreamDriverSeq int

func newFakeDreamDB(t *testing.T, behavior *fakeDreamDBBehavior) *sql.DB {
	t.Helper()
	fakeDreamDriverSeq++
	driverName := fmt.Sprintf("fakedb_dream_%d", fakeDreamDriverSeq)
	sql.Register(driverName, &fakeDreamDriver{behavior: behavior})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open fake dream db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestDreamTrackerRepositoryImplementsContract(t *testing.T) {
	var _ repository.DreamTrackerRepository = (*repository.DBDreamTrackerRepository)(nil)
}

func TestCreateDreamTrackerRepositorySeedsFundingRequirements(t *testing.T) {
	execCount := 0
	queryCount := 0
	fundingID := "funding-1"
	db := newFakeDreamDB(t, &fakeDreamDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			execCount++
			return fakeDreamResult{affected: 1}, nil
		},
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if strings.Contains(query, "FROM funding_requirements") {
				queryCount++
				return &fakeDreamRows{
					columns: []string{"req_catalog_id"},
					rows:    [][]driver.Value{{"req-1"}, {"req-2"}},
				}, nil
			}
			return &fakeDreamRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
		},
	})
	repo := repository.NewDreamTrackerRepository(db)

	now := time.Now().UTC()
	tracker, err := repo.CreateDreamTracker(context.Background(), model.DreamTracker{
		DreamTrackerID: "tracker-1",
		UserID:         "user-1",
		ProgramID:      "program-1",
		FundingID:      &fundingID,
		Title:          "Plan A",
		Status:         model.DreamTrackerStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceType:     "MANUAL",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if tracker.DreamTrackerID != "tracker-1" {
		t.Fatalf("expected tracker-1 got %s", tracker.DreamTrackerID)
	}
	if queryCount != 1 {
		t.Fatalf("expected 1 funding requirements query, got %d", queryCount)
	}
	if execCount != 3 {
		t.Fatalf("expected 3 exec calls (tracker + 2 statuses), got %d", execCount)
	}
}

func TestFindDreamTrackerDetailNotFoundRepository(t *testing.T) {
	db := newFakeDreamDB(t, &fakeDreamDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if strings.Contains(query, "FROM dream_tracker") {
				return &fakeDreamRows{
					columns: []string{"dream_tracker_id", "user_id", "program_id", "admission_id", "funding_id", "title", "status", "created_at", "updated_at", "source_type", "req_submission_id", "source_rec_result_id"},
					rows:    [][]driver.Value{},
				}, nil
			}
			return &fakeDreamRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
		},
	})
	repo := repository.NewDreamTrackerRepository(db)

	_, err := repo.FindDreamTrackerDetail(context.Background(), "tracker-x", "user-1")
	if err != errs.ErrDreamTrackerNotFound {
		t.Fatalf("expected %v, got %v", errs.ErrDreamTrackerNotFound, err)
	}
}

func TestFindDreamTrackerDetailRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeDreamDB(t, &fakeDreamDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			switch {
			case strings.Contains(query, "FROM dream_tracker"):
				return &fakeDreamRows{
					columns: []string{"dream_tracker_id", "user_id", "program_id", "admission_id", "funding_id", "title", "status", "created_at", "updated_at", "source_type", "req_submission_id", "source_rec_result_id"},
					rows: [][]driver.Value{{
						"tracker-1", "user-1", "program-1", "admission-1", "funding-1", "Plan A", string(model.DreamTrackerStatusActive), now, now, "MANUAL", "submission-1", "result-1",
					}},
				}, nil
			case strings.Contains(query, "FROM dream_requirement_status"):
				return &fakeDreamRows{
					columns: []string{"dream_req_status_id", "dream_tracker_id", "document_id", "req_catalog_id", "status", "notes", "ai_status", "ai_messages", "created_at"},
					rows: [][]driver.Value{{
						"req-status-1", "tracker-1", "doc-1", "req-1", string(model.DreamRequirementStatusUploaded), "looks good", "COMPLETED", "[\"ok\"]", now,
					}},
				}, nil
			default:
				return &fakeDreamRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
			}
		},
	})
	repo := repository.NewDreamTrackerRepository(db)

	detail, err := repo.FindDreamTrackerDetail(context.Background(), "tracker-1", "user-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if detail.DreamTracker.FundingID == nil || *detail.DreamTracker.FundingID != "funding-1" {
		t.Fatal("expected funding id to be set")
	}
	if len(detail.Requirements) != 1 || detail.Requirements[0].DocumentID == nil || *detail.Requirements[0].DocumentID != "doc-1" {
		t.Fatalf("unexpected requirements: %+v", detail.Requirements)
	}
}

func TestFindDocumentByIDAndUserRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeDreamDB(t, &fakeDreamDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if strings.Contains(query, "FROM documents") {
				return &fakeDreamRows{
					columns: []string{"document_id", "user_id", "nama", "original_filename", "storage_path", "dokumen_url", "public_url", "mime_type", "size_bytes", "dokumen_size_kb", "document_type", "uploaded_at"},
					rows: [][]driver.Value{{
						"doc-1", "user-1", "Transcript", "transcript.pdf", "/tmp/doc.pdf", "https://example.com/doc.pdf", "https://example.com/public.pdf", "application/pdf", int64(1024), int64(1), string(model.DocumentTypeTranscript), now,
					}},
				}, nil
			}
			return &fakeDreamRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
		},
	})
	repo := repository.NewDreamTrackerRepository(db)

	doc, err := repo.FindDocumentByIDAndUser(context.Background(), "doc-1", "user-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if doc.Nama != "Transcript" || doc.DokumenURL != "https://example.com/doc.pdf" {
		t.Fatalf("unexpected doc payload: %+v", doc)
	}
}

func TestFindDreamRequirementStatusByIDAndUserRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeDreamDB(t, &fakeDreamDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if strings.Contains(query, "FROM dream_requirement_status drs") {
				return &fakeDreamRows{
					columns: []string{"dream_req_status_id", "dream_tracker_id", "document_id", "req_catalog_id", "status", "notes", "ai_status", "ai_messages", "created_at"},
					rows: [][]driver.Value{{
						"req-status-1", "tracker-1", "doc-1", "req-1", string(model.DreamRequirementStatusUploaded), "uploaded", "PENDING", "[\"queued\"]", now,
					}},
				}, nil
			}
			return &fakeDreamRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
		},
	})
	repo := repository.NewDreamTrackerRepository(db)

	item, err := repo.FindDreamRequirementStatusByIDAndUser(context.Background(), "req-status-1", "user-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if item.DocumentID == nil || *item.DocumentID != "doc-1" {
		t.Fatalf("unexpected requirement payload: %+v", item)
	}
}

func TestUpdateDreamRequirementStatusRepository(t *testing.T) {
	db := newFakeDreamDB(t, &fakeDreamDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			if strings.Contains(query, "UPDATE dream_requirement_status") {
				return fakeDreamResult{affected: 1}, nil
			}
			return fakeDreamResult{affected: 0}, nil
		},
	})
	repo := repository.NewDreamTrackerRepository(db)
	note := "done"
	docID := "doc-1"
	aiStatus := "COMPLETED"
	aiMessages := "[\"ok\"]"

	err := repo.UpdateDreamRequirementStatus(context.Background(), model.DreamRequirementStatus{
		DreamReqStatusID: "req-status-1",
		DocumentID:       &docID,
		Status:           model.DreamRequirementStatusVerified,
		Notes:            &note,
		AIStatus:         &aiStatus,
		AIMessages:       &aiMessages,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
