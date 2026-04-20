ALTER TABLE recommendation_results
  DROP COLUMN IF EXISTS raw_recommendation_json,
  DROP COLUMN IF EXISTS scholarship_recommendations_json,
  DROP COLUMN IF EXISTS match_evidence_json,
  DROP COLUMN IF EXISTS preference_reasoning_json,
  DROP COLUMN IF EXISTS score_breakdown_json,
  DROP COLUMN IF EXISTS admission_difficulty,
  DROP COLUMN IF EXISTS overall_recommendation_score,
  DROP COLUMN IF EXISTS admission_chance_score;
