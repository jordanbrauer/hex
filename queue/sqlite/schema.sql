CREATE TABLE IF NOT EXISTS queue_messages (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    topic        TEXT    NOT NULL,
    body         BLOB    NOT NULL,
    attempts     INTEGER NOT NULL DEFAULT 0,
    enqueued_at  INTEGER NOT NULL,    -- unix millis
    deliver_at   INTEGER NOT NULL,    -- unix millis
    claimed_at   INTEGER,             -- unix millis, NULL when available
    claimed_by   TEXT,
    dedup_key    TEXT,
    metadata     TEXT
);

CREATE INDEX IF NOT EXISTS idx_queue_topic_deliver
    ON queue_messages (topic, deliver_at)
    WHERE claimed_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_queue_dedup
    ON queue_messages (topic, dedup_key)
    WHERE dedup_key IS NOT NULL;
