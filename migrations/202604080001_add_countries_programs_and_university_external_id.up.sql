CREATE TABLE IF NOT EXISTS countries (
  negara_id TEXT PRIMARY KEY,
  nama TEXT NOT NULL,
  nama_lokal TEXT NOT NULL,
  benua TEXT NOT NULL,
  mata_uang TEXT NOT NULL,
  bahasa_resmi TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE universities
  ADD COLUMN IF NOT EXISTS external_id TEXT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_universities_external_id
  ON universities(external_id)
  WHERE external_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS programs (
  program_id TEXT PRIMARY KEY,
  university_id UUID NOT NULL REFERENCES universities(id) ON DELETE CASCADE,
  nama_univ TEXT NOT NULL,
  nama TEXT NOT NULL,
  jenjang TEXT NOT NULL,
  bahasa TEXT NOT NULL,
  program_url TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_programs_university_id ON programs(university_id);
