package repository

import (
	"context"
	"database/sql"
	"fmt"
	"groupeak/internal/models"
	"strings"
)

type ProjectRepository interface {
	// Проверки
	IsProjectOwner(ctx context.Context, db DBTX, projectID, userID int64) (bool, error)
	IsProjectMember(ctx context.Context, db DBTX, projectID, userID int64) (bool, error)
	EnsureProjectExists(ctx context.Context, db DBTX, projectID int64) error
	GetProjectOwnerID(ctx context.Context, db DBTX, projectID int64) (int64, error)
	GetUserEmail(ctx context.Context, db DBTX, userID int64) (string, error)
	CheckUserIsMemberByEmail(ctx context.Context, db DBTX, projectID int64, email string) (bool, error)

	// CRUD Проектов
	CreateProject(ctx context.Context, db DBTX, p *models.Project) error
	UpdateProjectDynamic(ctx context.Context, db DBTX, projectID int64, setParts []string, args []interface{}) (*models.Project, error)
	DeleteProject(ctx context.Context, db DBTX, projectID int64) error
	GetProjectDetails(ctx context.Context, db DBTX, projectID int64) (*models.ProjectWithStats, error)
	ListUserProjects(ctx context.Context, db DBTX, userID int64) ([]models.Project, error)

	// Участники
	AddProjectMember(ctx context.Context, db DBTX, projectID, userID int64, role string) error
	RemoveProjectMember(ctx context.Context, db DBTX, projectID, userID int64) error
	RemoveMemberFromTasks(ctx context.Context, db DBTX, projectID, userID int64) error
	ListProjectMembers(ctx context.Context, db DBTX, projectID int64) ([]models.ProjectMemberWithUser, error)

	// Инвайты
	CheckPendingInvite(ctx context.Context, db DBTX, projectID int64, email string) (bool, error)
	CreateInvite(ctx context.Context, db DBTX, invite *models.ProjectInvite) error
	GetInviteByToken(ctx context.Context, db DBTX, token string) (*models.ProjectInvite, error)
	UpdateInviteStatus(ctx context.Context, db DBTX, inviteID int64, status string) (*models.ProjectInvite, error)
}

type projectRepo struct{}

func NewProjectRepository() ProjectRepository {
	return &projectRepo{}
}

func (r *projectRepo) IsProjectOwner(ctx context.Context, db DBTX, projectID, userID int64) (bool, error) {
	var ownerID int64
	err := db.QueryRowContext(ctx, `SELECT owner_id FROM projects WHERE id = $1`, projectID).Scan(&ownerID)
	if err != nil {
		return false, err
	}
	return ownerID == userID, nil
}

func (r *projectRepo) IsProjectMember(ctx context.Context, db DBTX, projectID, userID int64) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2)`, projectID, userID).Scan(&exists)
	return exists, err
}

func (r *projectRepo) EnsureProjectExists(ctx context.Context, db DBTX, projectID int64) error {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id = $1)`, projectID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}
	return nil
}

func (r *projectRepo) GetProjectOwnerID(ctx context.Context, db DBTX, projectID int64) (int64, error) {
	var ownerID int64
	err := db.QueryRowContext(ctx, `SELECT owner_id FROM projects WHERE id = $1`, projectID).Scan(&ownerID)
	return ownerID, err
}

func (r *projectRepo) GetUserEmail(ctx context.Context, db DBTX, userID int64) (string, error) {
	var email string
	err := db.QueryRowContext(ctx, `SELECT email FROM users WHERE id = $1`, userID).Scan(&email)
	return email, err
}

func (r *projectRepo) CheckUserIsMemberByEmail(ctx context.Context, db DBTX, projectID int64, email string) (bool, error) {
	var alreadyMember bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM project_members pm
			JOIN users u ON u.id = pm.user_id
			WHERE pm.project_id = $1 AND u.email = $2
		)`, projectID, email).Scan(&alreadyMember)
	return alreadyMember, err
}

func (r *projectRepo) CreateProject(ctx context.Context, db DBTX, p *models.Project) error {
	query := `
		INSERT INTO projects (owner_id, name, description, deadline_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at;
	`
	return db.QueryRowContext(ctx, query, p.OwnerID, p.Name, p.Description, p.DeadlineAt).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *projectRepo) UpdateProjectDynamic(ctx context.Context, db DBTX, projectID int64, setParts []string, args []interface{}) (*models.Project, error) {
	setParts = append(setParts, "updated_at = NOW()")
	args = append(args, projectID)
	query := fmt.Sprintf(`
		UPDATE projects
		SET %s
		WHERE id = $%d
		RETURNING id, owner_id, name, description, deadline_at, created_at, updated_at;
	`, strings.Join(setParts, ", "), len(args))

	var p models.Project
	var descField sql.NullString
	var deadlineField sql.NullTime

	err := db.QueryRowContext(ctx, query, args...).Scan(
		&p.ID, &p.OwnerID, &p.Name, &descField, &deadlineField, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if descField.Valid {
		v := descField.String
		p.Description = &v
	}
	if deadlineField.Valid {
		v := deadlineField.Time
		p.DeadlineAt = &v
	}
	return &p, nil
}

func (r *projectRepo) DeleteProject(ctx context.Context, db DBTX, projectID int64) error {
	_, err := db.ExecContext(ctx, `DELETE FROM projects WHERE id = $1`, projectID)
	return err
}

func (r *projectRepo) GetProjectDetails(ctx context.Context, db DBTX, projectID int64) (*models.ProjectWithStats, error) {
	query := `
		SELECT 
			p.id, p.owner_id, p.name, p.description, p.deadline_at, p.created_at, p.updated_at,
			(SELECT COUNT(*) FROM project_members WHERE project_id = p.id) AS member_count,
			(SELECT COUNT(*) FROM tasks WHERE project_id = p.id) AS task_count
		FROM projects p 
		WHERE p.id = $1;
	`
	var stats models.ProjectWithStats
	var descField sql.NullString
	var deadlineField sql.NullTime

	err := db.QueryRowContext(ctx, query, projectID).Scan(
		&stats.ID, &stats.OwnerID, &stats.Name, &descField, &deadlineField,
		&stats.CreatedAt, &stats.UpdatedAt, &stats.MemberCount, &stats.TaskCount,
	)
	if err != nil {
		return nil, err
	}

	if descField.Valid {
		v := descField.String
		stats.Description = &v
	}
	if deadlineField.Valid {
		v := deadlineField.Time
		stats.DeadlineAt = &v
	}
	return &stats, nil
}

func (r *projectRepo) ListUserProjects(ctx context.Context, db DBTX, userID int64) ([]models.Project, error) {
	query := `
		SELECT DISTINCT p.id, p.owner_id, p.name, p.description, p.deadline_at, p.created_at, p.updated_at
		FROM projects p
		LEFT JOIN project_members pm ON pm.project_id = p.id
		WHERE p.owner_id = $1 OR pm.user_id = $1
		ORDER BY p.created_at DESC;
	`
	rows, err := db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		var descField sql.NullString
		var deadlineField sql.NullTime

		if err := rows.Scan(&p.ID, &p.OwnerID, &p.Name, &descField, &deadlineField, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if descField.Valid {
			v := descField.String
			p.Description = &v
		}
		if deadlineField.Valid {
			v := deadlineField.Time
			p.DeadlineAt = &v
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *projectRepo) AddProjectMember(ctx context.Context, db DBTX, projectID, userID int64, role string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, $3)`, projectID, userID, role)
	return err
}

func (r *projectRepo) RemoveProjectMember(ctx context.Context, db DBTX, projectID, userID int64) error {
	_, err := db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`, projectID, userID)
	return err
}

func (r *projectRepo) RemoveMemberFromTasks(ctx context.Context, db DBTX, projectID, userID int64) error {
	_, err := db.ExecContext(ctx, `
		DELETE FROM task_assignees 
		WHERE user_id = $1 AND task_id IN (SELECT id FROM tasks WHERE project_id = $2)
	`, userID, projectID)
	return err
}

func (r *projectRepo) ListProjectMembers(ctx context.Context, db DBTX, projectID int64) ([]models.ProjectMemberWithUser, error) {
	query := `
		SELECT
			pm.id, pm.project_id, pm.user_id, pm.role, pm.created_at,
			COALESCE(u.full_name, '') as full_name, u.email,
			COALESCE(u.position, '') as position, u.avatar_url
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = $1
		ORDER BY CASE pm.role WHEN 'owner' THEN 0 ELSE 1 END, COALESCE(u.full_name, '');
	`
	rows, err := db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.ProjectMemberWithUser
	for rows.Next() {
		var member models.ProjectMemberWithUser
		var avatarField sql.NullString

		if err := rows.Scan(&member.ID, &member.ProjectID, &member.UserID, &member.Role, &member.CreatedAt, &member.FullName, &member.Email, &member.Position, &avatarField); err != nil {
			return nil, err
		}
		if avatarField.Valid {
			v := avatarField.String
			member.AvatarURL = &v
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *projectRepo) CheckPendingInvite(ctx context.Context, db DBTX, projectID int64, email string) (bool, error) {
	var id int64
	err := db.QueryRowContext(ctx, `SELECT id FROM project_invites WHERE project_id = $1 AND email = $2 AND status = 'pending'`, projectID, email).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (r *projectRepo) CreateInvite(ctx context.Context, db DBTX, invite *models.ProjectInvite) error {
	query := `
		INSERT INTO project_invites (project_id, email, token, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id, created_at, updated_at;
	`
	return db.QueryRowContext(ctx, query, invite.ProjectID, invite.Email, invite.Token).Scan(&invite.ID, &invite.CreatedAt, &invite.UpdatedAt)
}

func (r *projectRepo) GetInviteByToken(ctx context.Context, db DBTX, token string) (*models.ProjectInvite, error) {
	var invite models.ProjectInvite
	query := `SELECT id, project_id, email, token, status, created_at, updated_at FROM project_invites WHERE token = $1`
	err := db.QueryRowContext(ctx, query, token).Scan(
		&invite.ID, &invite.ProjectID, &invite.Email, &invite.Token, &invite.Status, &invite.CreatedAt, &invite.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *projectRepo) UpdateInviteStatus(ctx context.Context, db DBTX, inviteID int64, status string) (*models.ProjectInvite, error) {
	var invite models.ProjectInvite
	query := `
		UPDATE project_invites
		SET status = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, project_id, email, token, status, created_at, updated_at;
	`
	err := db.QueryRowContext(ctx, query, status, inviteID).Scan(
		&invite.ID, &invite.ProjectID, &invite.Email, &invite.Token, &invite.Status, &invite.CreatedAt, &invite.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &invite, nil
}
