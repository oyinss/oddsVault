-- 0022: match-result-edge v2.
--
-- v1 bucketed prices at 0.1. With the settled history we actually have, that
-- split the sample into ~200 buckets and almost no bucket could reach the
-- min_sample floor, so the strategy could never produce a pick. v2 widens the
-- bucket to 0.5, which keeps the sample floor meaningful while still keeping
-- prices apart. Rules are versioned, so v1 history stays readable.

INSERT INTO strategy_versions (strategy_id, version, rules)
SELECT id, 2, '{
  "market": "match_result",
  "min_sample": 20,
  "min_edge": 0.03,
  "max_picks": 5,
  "bucket": 0.5,
  "rationale": "0.1 buckets left no bucket above the sample floor; 0.5 groups comparable prices without merging favourites with longshots"
}'::jsonb
FROM strategy_definitions WHERE slug = 'match-result-edge'
ON CONFLICT (strategy_id, version) DO NOTHING;
