CREATE TABLE task_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id),
    device_id VARCHAR(255) NOT NULL,
    device_id_hash VARCHAR(64) NOT NULL,
    runner_address VARCHAR(42) NOT NULL,
    creator_address VARCHAR(42) NOT NULL,
    output TEXT,
    error TEXT,
    exit_code INT,
    execution_time BIGINT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT fk_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX idx_task_results_device_id ON task_results(device_id);
CREATE INDEX idx_task_results_device_id_hash ON task_results(device_id_hash);
CREATE INDEX idx_task_results_runner_address ON task_results(runner_address);
CREATE INDEX idx_task_results_creator_address ON task_results(creator_address); 