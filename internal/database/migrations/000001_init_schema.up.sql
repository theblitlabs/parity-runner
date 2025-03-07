-- Create tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY,
    creator_id UUID NOT NULL,
    creator_address VARCHAR(42) NOT NULL,
    creator_device_id VARCHAR(64) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL DEFAULT 'file',
    config JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(50) NOT NULL,
    reward DECIMAL(20,8) NOT NULL,
    runner_id UUID,
    environment JSONB,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP
);

-- Create indexes for tasks table
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_type ON tasks(type);
CREATE INDEX idx_tasks_creator_id ON tasks(creator_id);
CREATE INDEX idx_tasks_runner_id ON tasks(runner_id);
CREATE INDEX idx_tasks_creator_address ON tasks(creator_address);
CREATE INDEX idx_tasks_creator_device_id ON tasks(creator_device_id);

-- Create task_results table
CREATE TABLE task_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL,
    device_id VARCHAR(255) NOT NULL,
    device_id_hash VARCHAR(64) NOT NULL,
    runner_address VARCHAR(255) NOT NULL,
    creator_address VARCHAR(255) NOT NULL,
    creator_device_id TEXT,
    solver_device_id TEXT,
    output TEXT,
    error TEXT,
    exit_code INT,
    execution_time BIGINT,
    reward DECIMAL(20,8),
    ipfs_cid TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT fk_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Create indexes for task_results table
CREATE INDEX idx_task_results_task_id ON task_results(task_id);
CREATE INDEX idx_task_results_device_id ON task_results(device_id);
CREATE INDEX idx_task_results_device_id_hash ON task_results(device_id_hash);
CREATE INDEX idx_task_results_runner_address ON task_results(runner_address);
CREATE INDEX idx_task_results_creator_address ON task_results(creator_address);

-- Create webhooks table
CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    runner_id UUID NOT NULL,
    device_id TEXT NOT NULL,
    url TEXT NOT NULL,
    last_heartbeat TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_runner_id ON webhooks(runner_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_device_id ON webhooks(device_id);

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS webhook_id UUID REFERENCES webhooks(id) ON DELETE SET NULL;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS error TEXT; 