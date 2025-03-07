-- Drop task_results table and its indexes
DROP TABLE IF EXISTS task_results;

-- Drop tasks table and its indexes
DROP TABLE IF EXISTS tasks;

-- Drop webhooks table and its indexes
ALTER TABLE tasks DROP COLUMN IF EXISTS webhook_id;
ALTER TABLE tasks DROP COLUMN IF EXISTS error;

DROP INDEX IF EXISTS idx_webhooks_device_id;
DROP INDEX IF EXISTS idx_webhooks_runner_id;
DROP TABLE IF EXISTS webhooks;