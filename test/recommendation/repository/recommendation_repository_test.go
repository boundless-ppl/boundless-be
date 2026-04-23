package repository_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
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
	var insertResultArgs []driver.NamedValue
	db := newFakeRecDB(t, &fakeRecDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			execCount++
			if strings.Contains(query, "INSERT INTO recommendation_results") {
				insertResultArgs = append([]driver.NamedValue(nil), args...)
			}
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
		RecResultID:                    "res-1",
		ResultSetID:                    "set-1",
		ProgramID:                      stringPtr("program-1"),
		RankNo:                         1,
		UniversityName:                 "University A",
		ProgramName:                    "CS",
		Country:                        "Japan",
		FitScore:                       90,
		AdmissionChanceScore:           84,
		OverallRecommendationScore:     88,
		FitLevel:                       "high",
		AdmissionDifficulty:            "moderate",
		ScoreBreakdownJSON:             `{"academic_fit":90}`,
		Overview:                       "overview",
		WhyThisUniversity:              "why",
		WhyThisProgram:                 "why program",
		PreferenceReasoningJSON:        `["reason"]`,
		MatchEvidenceJSON:              `["evidence"]`,
		ScholarshipRecommendationsJSON: `[{"scholarship_name":"MEXT","funding_id":"fund-1"}]`,
		ReasonSummary:                  "summary",
		ProsJSON:                       `["pro"]`,
		ConsJSON:                       `["con"]`,
		RawRecommendationJSON:          `{"rank":1}`,
		CreatedAt:                      now,
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
	if len(insertResultArgs) != 24 {
		t.Fatalf("expected 24 insert args for expanded result payload, got %d", len(insertResultArgs))
	}
	if got := insertResultArgs[18].Value; got != `[{"scholarship_name":"MEXT","funding_id":"fund-1"}]` {
		t.Fatalf("expected scholarship payload to be persisted, got %#v", got)
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
					columns: []string{"rec_result_id", "result_set_id", "program_id", "admission_id", "rank_no", "university_name", "program_name", "country", "fit_score", "admission_chance_score", "overall_recommendation_score", "fit_level", "admission_difficulty", "score_breakdown_json", "overview", "why_this_university", "why_this_program", "preference_reasoning_json", "match_evidence_json", "scholarship_recommendations_json", "reason_summary", "pros_json", "cons_json", "created_at"},
					rows: [][]driver.Value{
						{"res-1", "set-1", "program-1", "admission-1", int64(1), "University A", "CS", "Japan", int64(90), int64(76), int64(88), "high", "moderate", `{"academic_fit":90}`, "overview", "why uni", "why program", `["reason"]`, `["evidence"]`, `[{"scholarship_name":"MEXT","funding_id":"fund-1"}]`, "summary", `["pro"]`, `["con"]`, now},
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
	if detail.Results[0].ScholarshipRecommendationsJSON != `[{"scholarship_name":"MEXT","funding_id":"fund-1"}]` {
		t.Fatalf("expected scholarship payload to round-trip, got %s", detail.Results[0].ScholarshipRecommendationsJSON)
	}
}

func stringPtr(value string) *string {
	return &value
}

func TestFindMatchingProgramsRepositoryUsesJSONPayload(t *testing.T) {
	db := newFakeRecDB(t, &fakeRecDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "jsonb_array_elements($1::jsonb)") {
				t.Fatalf("expected JSON-array based query, got %s", query)
			}
			if len(args) != 1 {
				t.Fatalf("expected one query arg, got %d", len(args))
			}
			payloadRaw, ok := args[0].Value.(string)
			if !ok {
				t.Fatalf("expected string payload, got %T", args[0].Value)
			}
			var payload []map[string]string
			if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
				t.Fatalf("expected valid JSON payload, got %v", err)
			}
			if len(payload) != 1 {
				t.Fatalf("expected deduplicated payload length 1, got %d", len(payload))
			}
			return &fakeRecRows{
				columns: []string{"program_id", "nama", "nama", "university_name", "program_name"},
				rows: [][]driver.Value{
					{"program-1", "Computer Science", "University A", "university a", "computer science"},
				},
			}, nil
		},
	})
	repo := repository.NewRecommendationRepository(db)

	out, err := repo.FindMatchingPrograms(context.Background(), []repository.RecommendationProgramLookup{
		{UniversityName: "University A", ProgramName: "Computer Science"},
		{UniversityName: " university   a ", ProgramName: " computer  science "},
		{UniversityName: "", ProgramName: "invalid"},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(out) != 1 || out[0].ProgramID != "program-1" {
		t.Fatalf("expected one match program-1, got %#v", out)
	}
}

func TestFindRecommendationAllowedCandidatesRepositoryUsesJSONPreferredCodes(t *testing.T) {
	db := newFakeRecDB(t, &fakeRecDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "WITH preferred_codes(code) AS") {
				t.Fatalf("expected preferred_codes CTE, got %s", query)
			}
			if !strings.Contains(query, "LIMIT $2") {
				t.Fatalf("expected parameterized limit, got %s", query)
			}
			if len(args) != 2 {
				t.Fatalf("expected two query args, got %d", len(args))
			}
			preferredRaw, ok := args[0].Value.(string)
			if !ok {
				t.Fatalf("expected preferred codes JSON string, got %T", args[0].Value)
			}
			var preferred []string
			if err := json.Unmarshal([]byte(preferredRaw), &preferred); err != nil {
				t.Fatalf("expected valid preferred JSON, got %v", err)
			}
			if len(preferred) != 1 || preferred[0] != "JP" {
				t.Fatalf("expected filtered preferred codes [JP], got %#v", preferred)
			}
			if limit, ok := args[1].Value.(int64); !ok || limit != 5 {
				t.Fatalf("expected limit arg 5, got %#v (%T)", args[1].Value, args[1].Value)
			}
			return &fakeRecRows{
				columns: []string{"program_id", "nama", "nama", "negara_id", "website_url", "website"},
				rows: [][]driver.Value{
					{"program-1", "Computer Science", "University A", "JP", "https://unia.example/cs", "https://unia.example"},
				},
			}, nil
		},
	})
	repo := repository.NewRecommendationRepository(db)

	out, err := repo.FindRecommendationAllowedCandidates(context.Background(), []string{"JP", " ", ""}, 5)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(out) != 1 || out[0].ProgramID != "program-1" {
		t.Fatalf("expected one candidate program-1, got %#v", out)
	}
}

func TestFindScholarshipMatchesRepositoryUsesJSONProgramIDs(t *testing.T) {
	db := newFakeRecDB(t, &fakeRecDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "jsonb_array_elements_text($1::jsonb)") {
				t.Fatalf("expected JSON-array based filter, got %s", query)
			}
			if len(args) != 1 {
				t.Fatalf("expected one query arg, got %d", len(args))
			}
			raw, ok := args[0].Value.(string)
			if !ok {
				t.Fatalf("expected string payload, got %T", args[0].Value)
			}
			var ids []string
			if err := json.Unmarshal([]byte(raw), &ids); err != nil {
				t.Fatalf("expected valid program ids JSON, got %v", err)
			}
			if len(ids) != 1 || ids[0] != "program-1" {
				t.Fatalf("expected filtered program ids [program-1], got %#v", ids)
			}
			return &fakeRecRows{
				columns: []string{"program_id", "nama_beasiswa", "funding_id", "admission_id"},
				rows: [][]driver.Value{
					{"program-1", "MEXT", "fund-1", "adm-1"},
				},
			}, nil
		},
	})
	repo := repository.NewRecommendationRepository(db)

	out, err := repo.FindScholarshipMatches(context.Background(), []string{"program-1", "", " "})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(out) != 1 || out[0].FundingID != "fund-1" {
		t.Fatalf("expected one scholarship match with fund-1, got %#v", out)
	}
}

func TestListRecommendationCountryCodesRepository(t *testing.T) {
	db := newFakeRecDB(t, &fakeRecDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "FROM universities") {
				t.Fatalf("expected query from universities, got %s", query)
			}
			return &fakeRecRows{
				columns: []string{"negara_id"},
				rows: [][]driver.Value{
					{"JP"},
					{" SG "},
				},
			}, nil
		},
	})
	repo := repository.NewRecommendationRepository(db)

	codes, err := repo.ListRecommendationCountryCodes(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(codes) != 2 || codes[0] != "JP" || codes[1] != "SG" {
		t.Fatalf("expected trimmed country codes [JP SG], got %#v", codes)
	}
}

func TestFindLatestCompletedSubmissionByTranscriptDocumentRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeRecDB(t, &fakeRecDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			switch {
			case strings.Contains(query, "FROM recommendation_submissions") &&
				strings.Contains(query, "AND cv_document_id IS NULL") &&
				strings.Contains(query, "LIMIT 1"):
				return &fakeRecRows{
					columns: []string{"rec_submission_id"},
					rows:    [][]driver.Value{{"submission-1"}},
				}, nil
			case strings.Contains(query, "FROM recommendation_submissions"):
				return &fakeRecRows{
					columns: []string{"rec_submission_id", "user_id", "transcript_document_id", "cv_document_id", "status", "created_at", "submitted_at"},
					rows: [][]driver.Value{
						{"submission-1", "user-1", "doc-transcript", nil, string(model.RecommendationStatusCompleted), now, now},
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
					columns: []string{"rec_result_id", "result_set_id", "program_id", "admission_id", "rank_no", "university_name", "program_name", "country", "fit_score", "admission_chance_score", "overall_recommendation_score", "fit_level", "admission_difficulty", "score_breakdown_json", "overview", "why_this_university", "why_this_program", "preference_reasoning_json", "match_evidence_json", "scholarship_recommendations_json", "reason_summary", "pros_json", "cons_json", "created_at"},
					rows: [][]driver.Value{
						{"res-1", "set-1", "program-1", nil, int64(1), "University A", "CS", "Japan", int64(90), int64(80), int64(88), "high", "moderate", `{"academic_fit":90}`, "overview", "why uni", "why program", `["reason"]`, `["evidence"]`, `[]`, "summary", `["pro"]`, `["con"]`, now},
					},
				}, nil
			default:
				return &fakeRecRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
			}
		},
	})
	repo := repository.NewRecommendationRepository(db)

	detail, err := repo.FindLatestCompletedSubmissionByTranscriptDocument(context.Background(), "user-1", "doc-transcript")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if detail.Submission.RecSubmissionID != "submission-1" {
		t.Fatalf("expected submission-1, got %s", detail.Submission.RecSubmissionID)
	}
	if len(detail.Results) != 1 {
		t.Fatalf("expected one recommendation result, got %d", len(detail.Results))
	}
}
