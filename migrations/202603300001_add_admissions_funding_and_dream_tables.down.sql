DROP INDEX IF EXISTS idx_dream_key_milestones_tracker_id;
DROP INDEX IF EXISTS idx_dream_requirement_status_tracker_id;
DROP INDEX IF EXISTS idx_dream_tracker_program_id;
DROP INDEX IF EXISTS idx_dream_tracker_user_id;
DROP INDEX IF EXISTS idx_funding_benefits_benefit_id;
DROP INDEX IF EXISTS idx_funding_benefits_funding_id;
DROP INDEX IF EXISTS idx_funding_requirements_req_catalog_id;
DROP INDEX IF EXISTS idx_funding_requirements_funding_id;
DROP INDEX IF EXISTS idx_admission_funding_funding_id;
DROP INDEX IF EXISTS idx_admission_funding_admission_id;
DROP INDEX IF EXISTS idx_admission_paths_program_id;

DROP TABLE IF EXISTS dream_key_milestones;
DROP TABLE IF EXISTS dream_requirement_status;
DROP TABLE IF EXISTS dream_tracker;
DROP TABLE IF EXISTS funding_benefits;
DROP TABLE IF EXISTS funding_requirements;
DROP TABLE IF EXISTS benefit_catalog;
DROP TABLE IF EXISTS requirement_catalog;
DROP TABLE IF EXISTS admission_funding;
DROP TABLE IF EXISTS funding_options;
DROP TABLE IF EXISTS admission_paths;

ALTER TABLE recommendation_results
  DROP COLUMN IF EXISTS score,
  DROP COLUMN IF EXISTS program_id;

ALTER TABLE recommendation_result_sets
  DROP COLUMN IF EXISTS submission_id;

ALTER TABLE recommendation_submissions
  DROP COLUMN IF EXISTS transkrip_dokumen_id,
  DROP COLUMN IF EXISTS cv_dokumen_id;

ALTER TABLE documents
  DROP COLUMN IF EXISTS dokumen_size_kb,
  DROP COLUMN IF EXISTS dokumen_url,
  DROP COLUMN IF EXISTS nama;
