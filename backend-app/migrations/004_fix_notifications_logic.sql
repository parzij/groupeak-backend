-- +goose Up
-- ИСПРАВЛЕНИЕ: Меняем структуру под полиморфную связь и JSONB payload
ALTER TABLE notifications
    DROP COLUMN IF EXISTS title,
    DROP COLUMN IF EXISTS message,
    DROP COLUMN IF EXISTS position,
    DROP COLUMN IF EXISTS is_approved,
    DROP COLUMN IF EXISTS task_id,
    
    ADD COLUMN IF NOT EXISTS type VARCHAR(50) NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS actor_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS entity_id BIGINT,
    ADD COLUMN IF NOT EXISTS payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS is_read BOOLEAN NOT NULL DEFAULT FALSE;

-- ИСПРАВЛЕНИЕ 1: user_id NOT NULL и ON DELETE CASCADE
ALTER TABLE notifications
    ALTER COLUMN user_id TYPE BIGINT,
    ALTER COLUMN user_id SET NOT NULL,
    DROP CONSTRAINT IF EXISTS notifications_user_id_fkey,
    ADD CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- ИСПРАВЛЕНИЕ 2: Убираем Foreign Key для project_id
ALTER TABLE notifications
    ALTER COLUMN project_id TYPE BIGINT,
    DROP CONSTRAINT IF EXISTS notifications_project_id_fkey;

CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(type);
CREATE INDEX IF NOT EXISTS idx_notifications_is_read ON notifications(is_read);


-- +goose Down
DROP INDEX IF EXISTS idx_notifications_type;
DROP INDEX IF EXISTS idx_notifications_is_read;

ALTER TABLE notifications
    DROP COLUMN IF EXISTS type,
    DROP COLUMN IF EXISTS actor_id,
    DROP COLUMN IF EXISTS entity_id,
    DROP COLUMN IF EXISTS payload,
    DROP COLUMN IF EXISTS is_read,

    ADD COLUMN title VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN message VARCHAR(100),
    ADD COLUMN position VARCHAR(16),
    ADD COLUMN is_approved BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN task_id INT REFERENCES tasks(id) ON DELETE SET NULL;

-- Возвращаем старые констрейнты
ALTER TABLE notifications
    ALTER COLUMN user_id DROP NOT NULL,
    DROP CONSTRAINT IF EXISTS notifications_user_id_fkey,
    ADD CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE notifications
    ADD CONSTRAINT notifications_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;