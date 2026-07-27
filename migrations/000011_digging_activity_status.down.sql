-- Rollback: 000011_digging_activity_status
ALTER TABLE loading_logs
    DROP COLUMN IF EXISTS activity_status,
    DROP COLUMN IF EXISTS digging_confidence;

DROP INDEX IF EXISTS idx_loading_logs_activity_status;
