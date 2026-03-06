CREATE TABLE IF NOT EXISTS universities (
  id UUID PRIMARY KEY,
  negara_id TEXT NOT NULL,
  nama TEXT NOT NULL,
  kota TEXT NOT NULL,
  tipe TEXT NOT NULL,
  deskripsi TEXT,
  website TEXT,
  ranking INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);