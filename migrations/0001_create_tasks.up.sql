CREATE TABLE IF NOT EXISTS task_templates (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL CHECK (type IN ('daily', 'monthly', 'parity', 'specific_days')),
    interval INT DEFAULT 0,
    day_of_month INT,
    parity TEXT CHECK (parity IS NULL OR parity IN ('even', 'odd')),
    specific_days TIMESTAMPTZ[], 
    is_active BOOLEAN DEFAULT true,
	starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	last_generated_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_task_templates_active ON task_templates (is_active);


CREATE TABLE IF NOT EXISTS tasks (
    id BIGSERIAL PRIMARY KEY,
    parent_id BIGINT REFERENCES task_templates(id) ON DELETE CASCADE, 
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    due_date TIMESTAMPTZ NOT NULL DEFAULT NOW(), 
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks (status);
CREATE INDEX IF NOT EXISTS idx_tasks_due_date ON tasks (due_date); 