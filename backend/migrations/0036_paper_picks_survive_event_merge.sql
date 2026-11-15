-- 0036: a paper pick must outlive the event row it was recorded on.
--
-- The 1xBet dedupe job merges duplicate fixtures by deleting the losing row.
-- With ON DELETE CASCADE that silently destroyed recorded picks. The pick is
-- training data: keep it, and let dedupe remap it to the surviving event.

ALTER TABLE paper_picks ALTER COLUMN event_id DROP NOT NULL;

ALTER TABLE paper_picks DROP CONSTRAINT paper_picks_event_id_fkey;

ALTER TABLE paper_picks
    ADD CONSTRAINT paper_picks_event_id_fkey
    FOREIGN KEY (event_id) REFERENCES events (id) ON DELETE SET NULL;
