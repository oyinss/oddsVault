-- 0037: mark reconstructed paper picks.
--
-- A dedupe cascade deleted one recorded pick before the foreign key was fixed.
-- The row is restored from the corner snapshot that was current when it was
-- recorded, and flagged so a training set never mistakes a reconstruction for
-- an untouched recording.

ALTER TABLE paper_picks ADD COLUMN recovered_at timestamptz;
ALTER TABLE paper_picks ADD COLUMN recovery_note text;
