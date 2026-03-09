CREATE TABLE IF NOT EXISTS documents (
  document_id UUID PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  original_filename TEXT NOT NULL,
  storage_path TEXT NOT NULL,
  public_url TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  document_type TEXT NOT NULL,
  uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recommendation_submissions (
  rec_submission_id UUID PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  transcript_document_id UUID NULL REFERENCES documents(document_id) ON DELETE SET NULL,
  cv_document_id UUID NULL REFERENCES documents(document_id) ON DELETE SET NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  submitted_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS recommendation_preferences (
  pref_id UUID PRIMARY KEY,
  rec_submission_id UUID NOT NULL REFERENCES recommendation_submissions(rec_submission_id) ON DELETE CASCADE,
  pref_key TEXT NOT NULL,
  pref_value TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recommendation_result_sets (
  result_set_id UUID PRIMARY KEY,
  rec_submission_id UUID NOT NULL REFERENCES recommendation_submissions(rec_submission_id) ON DELETE CASCADE,
  version_no INTEGER NOT NULL,
  generated_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recommendation_results (
  rec_result_id UUID PRIMARY KEY,
  result_set_id UUID NOT NULL REFERENCES recommendation_result_sets(result_set_id) ON DELETE CASCADE,
  rank_no INTEGER NOT NULL,
  university_name TEXT NOT NULL,
  program_name TEXT NOT NULL,
  country TEXT NOT NULL,
  fit_score INTEGER NOT NULL,
  fit_level TEXT NOT NULL,
  overview TEXT NOT NULL,
  why_this_university TEXT NOT NULL,
  why_this_program TEXT NOT NULL,
  reason_summary TEXT NOT NULL,
  pros_json JSONB NOT NULL,
  cons_json JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_documents_user_id ON documents(user_id);
CREATE INDEX IF NOT EXISTS idx_documents_type ON documents(document_type);
CREATE INDEX IF NOT EXISTS idx_rec_submissions_user_id ON recommendation_submissions(user_id);
CREATE INDEX IF NOT EXISTS idx_rec_preferences_submission_id ON recommendation_preferences(rec_submission_id);
CREATE INDEX IF NOT EXISTS idx_rec_result_sets_submission_id ON recommendation_result_sets(rec_submission_id);
CREATE INDEX IF NOT EXISTS idx_rec_results_result_set_id ON recommendation_results(result_set_id);
