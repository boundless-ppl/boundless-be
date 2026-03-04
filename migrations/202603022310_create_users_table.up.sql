CREATE TABLE IF NOT EXISTS users (
  user_id TEXT PRIMARY KEY,
  nama_lengkap TEXT NOT NULL,
  role TEXT NOT NULL,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  failed_login_count INTEGER NOT NULL DEFAULT 0,
  first_failed_at TIMESTAMPTZ NULL,
  locked_until TIMESTAMPTZ NULL
);
