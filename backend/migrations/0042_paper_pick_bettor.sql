-- 0042: record who played a paper pick.
--
-- Paper picks started as board recordings with no owner. They now also capture
-- live slips that were actually played, so the same attribution the journal uses
-- applies: emife, oyins, agent, or empty for an unattributed board recording.
ALTER TABLE paper_picks
    ADD COLUMN placed_by text;

COMMENT ON COLUMN paper_picks.placed_by IS
    'Who played it: emife, oyins or agent. NULL for a board recording that belongs to no slip.';
