CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY,
    creator_id VARCHAR(255) NOT NULL,
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

CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_type ON tasks(type);
CREATE INDEX idx_tasks_creator_id ON tasks(creator_id);
CREATE INDEX idx_tasks_runner_id ON tasks(runner_id); 