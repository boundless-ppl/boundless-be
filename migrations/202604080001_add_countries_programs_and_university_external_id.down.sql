DROP INDEX IF EXISTS idx_programs_university_id;
DROP TABLE IF EXISTS programs;

DROP INDEX IF EXISTS idx_universities_external_id;
ALTER TABLE universities DROP COLUMN IF EXISTS external_id;

DROP TABLE IF EXISTS countries;
