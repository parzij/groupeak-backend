-- +goose Up

ALTER TABLE notifications
    DROP COLUMN IF EXISTS title,
    DROP COLUMN IF EXISTS message,
    DROP COLUMN IF EXISTS position,
    DROP COLUMN IF EXISTS is_approved,
    DROP COLUMN IF EXISTS task_id,
    
    -- type: определяет тип события (например: 'task_updated', 'user_invited')
    ADD COLUMN type VARCHAR(50) NOT NULL,
    
    -- actor_id: ID пользователя, который совершил действие. 
    ADD COLUMN actor_id INT REFERENCES users(id) ON DELETE SET NULL,
    
    -- entity_id: ID сущности, с которой произошло действие
    ADD COLUMN entity_id INT,
    
    -- payload: сырые данные в формате JSONB
    ADD COLUMN payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    
    -- is_read: флаг прочитанности уведомления
    ADD COLUMN is_read BOOLEAN NOT NULL DEFAULT FALSE;

-- Индексы для быстрого поиска
CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(type);
CREATE INDEX IF NOT EXISTS idx_notifications_is_read ON notifications(is_read);

-- +goose Down
