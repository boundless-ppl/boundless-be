ALTER TABLE recommendation_results
  ADD COLUMN IF NOT EXISTS admission_chance_score INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS overall_recommendation_score INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS admission_difficulty TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS score_breakdown_json TEXT NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS preference_reasoning_json TEXT NOT NULL DEFAULT '[]',
  ADD COLUMN IF NOT EXISTS match_evidence_json TEXT NOT NULL DEFAULT '[]',
  ADD COLUMN IF NOT EXISTS scholarship_recommendations_json TEXT NOT NULL DEFAULT '[]',
  ADD COLUMN IF NOT EXISTS raw_recommendation_json TEXT NOT NULL DEFAULT '{}';

UPDATE recommendation_results
SET overall_recommendation_score = COALESCE(NULLIF(overall_recommendation_score, 0), score, fit_score)
WHERE overall_recommendation_score = 0;
