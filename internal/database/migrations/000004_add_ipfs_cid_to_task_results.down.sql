-- Remove columns from task_results table
ALTER TABLE task_results DROP COLUMN ipfs_cid;
ALTER TABLE task_results DROP COLUMN creator_device_id;
ALTER TABLE task_results DROP COLUMN solver_device_id;
ALTER TABLE task_results DROP COLUMN reward; 