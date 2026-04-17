-- +goose Up

-- 1. Таблица пользователей
CREATE TABLE IF NOT EXISTS users (
    id             SERIAL PRIMARY KEY,
    full_name      VARCHAR(150) NOT NULL,
    avatar_url     VARCHAR(2048),
    email          VARCHAR(255) NOT NULL UNIQUE,
    password_hash  VARCHAR(255) NOT NULL,
    position       VARCHAR(25) CHECK (
        position IS NULL OR position IN (
            'Frontend', 'Backend', 'Designer', 'QA', 'DevOps', 
            'Android', 'iOS', 'Data Analyst', 'Product', 'HR'
        )
    ),
    birth_date     DATE,
    about          VARCHAR(255),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Таблица проектов
CREATE TABLE IF NOT EXISTS projects (
    id          SERIAL PRIMARY KEY,
    owner_id    INT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    name        VARCHAR(150) NOT NULL,
    description VARCHAR(255),
    deadline_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Таблица задач
CREATE TABLE IF NOT EXISTS tasks (
    id                   SERIAL PRIMARY KEY,
    project_id           INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title                VARCHAR(200) NOT NULL,
    description          VARCHAR(255),
    comments             VARCHAR(255),
    status               VARCHAR(16) NOT NULL DEFAULT 'todo'
        CHECK (status IN ('todo', 'in_progress', 'in_review', 'done', 'postponed')),
    status_before_review VARCHAR(16),
    priority             VARCHAR(8) NOT NULL DEFAULT 'medium'
        CHECK (priority IN ('low', 'medium', 'high')),
    due_at               TIMESTAMPTZ,
    submitted_at         TIMESTAMPTZ,
    reviewed_at          TIMESTAMPTZ,
    reviewed_by          INT REFERENCES users(id) ON DELETE SET NULL,
    review_comment       VARCHAR(255),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON tasks(project_id);

-- 4. Связка задач и исполнителей
CREATE TABLE IF NOT EXISTS task_assignees (
    task_id    INT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id    INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (task_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_task_assignees_user_id ON task_assignees(user_id);

-- 5. Таблица уведомлений
CREATE TABLE IF NOT EXISTS notifications (
    id          SERIAL PRIMARY KEY,
    project_id  INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id     INT REFERENCES users(id) ON DELETE SET NULL,
    task_id     INT REFERENCES tasks(id) ON DELETE SET NULL,
    title       VARCHAR(200) NOT NULL,
    message     VARCHAR(100),
    position    VARCHAR(16), 
    is_approved BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notifications_project_id ON notifications(project_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_task_id ON notifications(task_id);

-- 6. Связка проектов и участников
CREATE TABLE IF NOT EXISTS project_members (
    id         SERIAL PRIMARY KEY,
    project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       VARCHAR(8) NOT NULL CHECK (role IN ('owner', 'member')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_members_unique_member UNIQUE (project_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_members_project_id ON project_members(project_id);
CREATE INDEX IF NOT EXISTS idx_project_members_user_id ON project_members(user_id);

-- 7. Приглашения в проект
CREATE TABLE IF NOT EXISTS project_invites (
    id         SERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    email      VARCHAR(255) NOT NULL,
    token      VARCHAR(255) NOT NULL UNIQUE,
    status     VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'cancelled', 'expired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_invites_project_id ON project_invites(project_id);
CREATE INDEX IF NOT EXISTS idx_project_invites_email ON project_invites(email);

-- +goose Down

DROP TABLE IF EXISTS project_invites;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS task_assignees;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS users;