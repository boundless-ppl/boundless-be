CREATE TABLE IF NOT EXISTS admission_paths (
  admission_id UUID PRIMARY KEY,
  program_id TEXT NOT NULL,
  nama TEXT NOT NULL,
  intake TEXT NOT NULL,
  deadline TIMESTAMPTZ NULL,
  requires_supervisor BOOLEAN NOT NULL DEFAULT FALSE,
  website_url TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS funding_options (
  funding_id UUID PRIMARY KEY,
  nama_beasiswa TEXT NOT NULL,
  deskripsi TEXT NULL,
  provider TEXT NOT NULL,
  tipe_pembiayaan TEXT NOT NULL CHECK (tipe_pembiayaan IN ('SCHOLARSHIP', 'SELF_FUNDED', 'ASSISTANTSHIP', 'LOAN', 'SPONSORSHIP')),
  website TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS admission_funding (
  admission_funding_id UUID PRIMARY KEY,
  admission_id UUID NOT NULL REFERENCES admission_paths(admission_id) ON DELETE CASCADE,
  funding_id UUID NOT NULL REFERENCES funding_options(funding_id) ON DELETE CASCADE,
  linkage_type TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS requirement_catalog (
  req_catalog_id UUID PRIMARY KEY,
  key TEXT NOT NULL UNIQUE,
  label TEXT NOT NULL,
  kategori TEXT NOT NULL,
  deskripsi TEXT NULL
);

CREATE TABLE IF NOT EXISTS benefit_catalog (
  benefit_id UUID PRIMARY KEY,
  key TEXT NOT NULL UNIQUE,
  label TEXT NOT NULL,
  kategori TEXT NOT NULL,
  deskripsi TEXT NULL
);

CREATE TABLE IF NOT EXISTS funding_requirements (
  funding_req_id UUID PRIMARY KEY,
  funding_id UUID NOT NULL REFERENCES funding_options(funding_id) ON DELETE CASCADE,
  req_catalog_id UUID NOT NULL REFERENCES requirement_catalog(req_catalog_id) ON DELETE CASCADE,
  is_required BOOLEAN NOT NULL DEFAULT FALSE,
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS funding_benefits (
  funding_benefit_id UUID PRIMARY KEY,
  funding_id UUID NOT NULL REFERENCES funding_options(funding_id) ON DELETE CASCADE,
  benefit_id UUID NOT NULL REFERENCES benefit_catalog(benefit_id) ON DELETE CASCADE,
  value_text TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);

ALTER TABLE documents
  ADD COLUMN IF NOT EXISTS nama TEXT NULL,
  ADD COLUMN IF NOT EXISTS dokumen_url TEXT NULL,
  ADD COLUMN IF NOT EXISTS dokumen_size_kb BIGINT NULL;

ALTER TABLE recommendation_submissions
  ADD COLUMN IF NOT EXISTS cv_dokumen_id UUID NULL REFERENCES documents(document_id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS transkrip_dokumen_id UUID NULL REFERENCES documents(document_id) ON DELETE SET NULL;

ALTER TABLE recommendation_result_sets
  ADD COLUMN IF NOT EXISTS submission_id TEXT NULL;

ALTER TABLE recommendation_results
  ADD COLUMN IF NOT EXISTS program_id TEXT NULL,
  ADD COLUMN IF NOT EXISTS score INTEGER NULL;

CREATE TABLE IF NOT EXISTS dream_tracker (
  dream_tracker_id UUID PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  program_id TEXT NOT NULL,
  admission_id UUID NULL REFERENCES admission_paths(admission_id) ON DELETE SET NULL,
  funding_id UUID NULL REFERENCES funding_options(funding_id) ON DELETE SET NULL,
  title TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  source_type TEXT NOT NULL,
  req_submission_id UUID NULL REFERENCES recommendation_submissions(rec_submission_id) ON DELETE SET NULL,
  source_rec_result_id UUID NULL REFERENCES recommendation_results(rec_result_id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS dream_requirement_status (
  dream_req_status_id UUID PRIMARY KEY,
  dream_tracker_id UUID NOT NULL REFERENCES dream_tracker(dream_tracker_id) ON DELETE CASCADE,
  document_id UUID NULL REFERENCES documents(document_id) ON DELETE SET NULL,
  req_catalog_id UUID NOT NULL REFERENCES requirement_catalog(req_catalog_id) ON DELETE RESTRICT,
  status TEXT NOT NULL,
  notes TEXT NULL,
  ai_status TEXT NULL,
  ai_messages TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS dream_key_milestones (
  dream_milestone_id UUID PRIMARY KEY,
  dream_tracker_id UUID NOT NULL REFERENCES dream_tracker(dream_tracker_id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  description TEXT NULL,
  deadline_date TIMESTAMPTZ NULL,
  is_required BOOLEAN NOT NULL DEFAULT FALSE,
  status TEXT NOT NULL CHECK (status IN ('NOT_STARTED', 'DONE', 'MISSED')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admission_paths_program_id ON admission_paths(program_id);
CREATE INDEX IF NOT EXISTS idx_admission_funding_admission_id ON admission_funding(admission_id);
CREATE INDEX IF NOT EXISTS idx_admission_funding_funding_id ON admission_funding(funding_id);
CREATE INDEX IF NOT EXISTS idx_funding_requirements_funding_id ON funding_requirements(funding_id);
CREATE INDEX IF NOT EXISTS idx_funding_requirements_req_catalog_id ON funding_requirements(req_catalog_id);
CREATE INDEX IF NOT EXISTS idx_funding_benefits_funding_id ON funding_benefits(funding_id);
CREATE INDEX IF NOT EXISTS idx_funding_benefits_benefit_id ON funding_benefits(benefit_id);
CREATE INDEX IF NOT EXISTS idx_dream_tracker_user_id ON dream_tracker(user_id);
CREATE INDEX IF NOT EXISTS idx_dream_tracker_program_id ON dream_tracker(program_id);
CREATE INDEX IF NOT EXISTS idx_dream_requirement_status_tracker_id ON dream_requirement_status(dream_tracker_id);
CREATE INDEX IF NOT EXISTS idx_dream_key_milestones_tracker_id ON dream_key_milestones(dream_tracker_id);
