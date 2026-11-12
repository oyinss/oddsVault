-- 0009: versioned settlement.
--
-- A settlement is never overwritten silently: the current row is the latest
-- accepted evaluation, and every evaluation is appended to the versions table.

CREATE TABLE selection_settlements (
    selection_id       bigint PRIMARY KEY REFERENCES market_selections (id) ON DELETE CASCADE,
    status             text NOT NULL,
    actual_value       text,
    result_detail      text,
    reason_code        text,
    data_source        text,
    source_updated_at  timestamptz,
    evaluated_at       timestamptz NOT NULL DEFAULT now(),
    rule_version       integer NOT NULL DEFAULT 1,
    evaluation_version integer NOT NULL DEFAULT 1,
    CONSTRAINT selection_settlements_status_check CHECK (status IN (
        'hit', 'miss', 'push', 'void', 'needs_data', 'pending', 'not_gradable'
    ))
);

CREATE INDEX selection_settlements_status_idx ON selection_settlements (status);
CREATE INDEX selection_settlements_reason_idx ON selection_settlements (reason_code);

CREATE TABLE selection_settlement_versions (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    selection_id       bigint NOT NULL REFERENCES market_selections (id) ON DELETE CASCADE,
    status             text NOT NULL,
    actual_value       text,
    result_detail      text,
    reason_code        text,
    data_source        text,
    source_updated_at  timestamptz,
    evaluated_at       timestamptz NOT NULL,
    rule_version       integer NOT NULL,
    evaluation_version integer NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT selection_settlement_versions_unique
        UNIQUE (selection_id, evaluation_version)
);

CREATE INDEX selection_settlement_versions_selection_idx
    ON selection_settlement_versions (selection_id, evaluation_version DESC);
