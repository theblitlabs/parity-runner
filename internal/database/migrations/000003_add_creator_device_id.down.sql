DROP INDEX IF EXISTS idx_tasks_creator_device_id;
ALTER TABLE tasks DROP COLUMN creator_device_id;