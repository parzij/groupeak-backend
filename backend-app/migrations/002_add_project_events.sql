-- +goose Up

-- Основная таблица мероприятий
CREATE TABLE IF NOT EXISTS project_events (
    id          SERIAL PRIMARY KEY,
    project_id  INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title       VARCHAR(100) NOT NULL,
    meeting_url VARCHAR(255),       
    description VARCHAR(255),                       
    start_at    TIMESTAMPTZ NOT NULL,        
    end_at      TIMESTAMPTZ,                 
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_events_project_id ON project_events(project_id);

-- Таблица участников мероприятия (многие-ко-многим)
CREATE TABLE IF NOT EXISTS event_participants (
    event_id   INT NOT NULL REFERENCES project_events(id) ON DELETE CASCADE,
    user_id    INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_event_participants_user_id ON event_participants(user_id);

-- +goose Down

DROP TABLE IF EXISTS event_participants;
DROP TABLE IF EXISTS project_events;