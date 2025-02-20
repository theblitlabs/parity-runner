ALTER TABLE tasks ADD COLUMN creator_device_id VARCHAR(64) NOT NULL DEFAULT '';
CREATE INDEX idx_tasks_creator_device_id ON tasks(creator_device_id);