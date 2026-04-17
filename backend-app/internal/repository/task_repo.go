package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"groupeak/internal/dto"
	"groupeak/internal/models"
)

type TaskRepository interface {
	// Валидация и проверки
	GetProjectOwnerID(ctx context.Context, db DBTX, projectID int64) (int64, error)
	CheckProjectExists(ctx context.Context, db DBTX, projectID int64) (bool, error)
	CheckProjectMember(ctx context.Context, db DBTX, projectID, userID int64) (bool, error)
	CheckTaskAssignee(ctx context.Context, db DBTX, taskID, userID int64) (bool, error)
	GetUsersExistence(ctx context.Context, db DBTX, ids []int64) (map[int64]struct{}, error)
	GetProjectMembersExistence(ctx context.Context, db DBTX, projectID int64, ids []int64) (map[int64]struct{}, error)
	CheckUserHasAnyProjectAccess(ctx context.Context, db DBTX, userID int64) (bool, error)

	// CRUD Задач
	CreateTask(ctx context.Context, db DBTX, t *models.Task) error
	GetTaskByID(ctx context.Context, db DBTX, taskID int64) (*models.Task, error)
	UpdateTaskDynamic(ctx context.Context, db DBTX, taskID int64, setParts []string, args []interface{}) (*models.Task, error)
	DeleteTask(ctx context.Context, db DBTX, taskID int64) error

	// Исполнители (Assignees)
	AddAssignee(ctx context.Context, db DBTX, taskID, userID int64) error
	ClearAssignees(ctx context.Context, db DBTX, taskID int64) error
	GetAssigneesForTask(ctx context.Context, db DBTX, taskID int64) ([]int64, error)
	GetAssigneesForTasks(ctx context.Context, db DBTX, taskIDs []int64) (map[int64][]int64, error)

	// Списки задач
	ListProjectTasks(ctx context.Context, db DBTX, projectID, limit, offset int64) ([]models.Task, []int64, error)
	ListTasksWithFilter(ctx context.Context, db DBTX, userID int64, filter dto.TaskFilterRequest) ([]models.Task, []int64, error)
	ListTasksByAssignees(ctx context.Context, db DBTX, projectID int64, assignees []int64) ([]models.Task, []int64, error)
	ListTasksByCategory(ctx context.Context, db DBTX, projectID int64, category models.TaskCategory) ([]models.Task, []int64, error)
	ListNearestTasks(ctx context.Context, db DBTX, userID int64) ([]models.Task, []int64, error)
	GetMyTasks(ctx context.Context, db DBTX, userID int64) ([]models.Task, []int64, error)

	// Workflow
	GetTaskProjectAndStatus(ctx context.Context, db DBTX, taskID int64) (int64, string, error)
	UpdateTaskStatus(ctx context.Context, db DBTX, taskID int64, status string) error
	SubmitForReview(ctx context.Context, db DBTX, taskID, projectID, userID int64) (int64, error)
	ReviewTaskApprove(ctx context.Context, db DBTX, taskID, projectID, ownerID int64, comment *string) (int64, error)
	ReviewTaskReject(ctx context.Context, db DBTX, taskID, projectID, ownerID int64, comment *string, newDueAt *time.Time) (int64, error)
}

type taskRepo struct{}

func NewTaskRepository() TaskRepository {
	return &taskRepo{}
}

// Вспомогательная функция для генерации плейсхолдеров
func makePlaceholders(start, count int) string {
	var b strings.Builder
	for i := 0; i < count; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('$')
		b.WriteString(strconv.Itoa(start + i))
	}
	return b.String()
}

// Вспомогательная функция для парсинга задачи из БД
func scanTask(scanner interface {
	Scan(dest ...interface{}) error
}) (models.Task, error) {
	var t models.Task
	var descField, commentsField sql.NullString
	var dueField sql.NullTime

	err := scanner.Scan(
		&t.ID, &t.ProjectID, &t.Title, &descField, &commentsField,
		&t.Status, &t.Priority, &dueField, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return t, err
	}
	if descField.Valid {
		t.Description = &descField.String
	}
	if commentsField.Valid {
		t.Comments = &commentsField.String
	}
	if dueField.Valid {
		t.DueAt = &dueField.Time
	}
	t.AssigneeIDs = []int64{}
	return t, nil
}

func (r *taskRepo) GetProjectOwnerID(ctx context.Context, db DBTX, projectID int64) (int64, error) {
	var ownerID int64
	err := db.QueryRowContext(ctx, `SELECT owner_id FROM projects WHERE id = $1`, projectID).Scan(&ownerID)
	return ownerID, err
}

func (r *taskRepo) CheckProjectExists(ctx context.Context, db DBTX, projectID int64) (bool, error) {
	var id int64
	err := db.QueryRowContext(ctx, `SELECT id FROM projects WHERE id = $1`, projectID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (r *taskRepo) CheckProjectMember(ctx context.Context, db DBTX, projectID, userID int64) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2)`, projectID, userID).Scan(&exists)
	return exists, err
}

func (r *taskRepo) CheckTaskAssignee(ctx context.Context, db DBTX, taskID, userID int64) (bool, error) {
	var isAssignee bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM task_assignees WHERE task_id = $1 AND user_id = $2)`, taskID, userID).Scan(&isAssignee)
	return isAssignee, err
}

func (r *taskRepo) GetUsersExistence(ctx context.Context, db DBTX, ids []int64) (map[int64]struct{}, error) {
	ph := makePlaceholders(1, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := db.QueryContext(ctx, `SELECT id FROM users WHERE id IN (`+ph+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	found := make(map[int64]struct{}, len(ids))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			found[id] = struct{}{}
		}
	}
	return found, rows.Err()
}

func (r *taskRepo) GetProjectMembersExistence(ctx context.Context, db DBTX, projectID int64, ids []int64) (map[int64]struct{}, error) {
	ph := makePlaceholders(2, len(ids))
	args := make([]interface{}, 0, 1+len(ids))
	args = append(args, projectID)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := db.QueryContext(ctx, `SELECT user_id FROM project_members WHERE project_id = $1 AND user_id IN (`+ph+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	found := make(map[int64]struct{}, len(ids))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			found[id] = struct{}{}
		}
	}
	return found, rows.Err()
}

func (r *taskRepo) CheckUserHasAnyProjectAccess(ctx context.Context, db DBTX, userID int64) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM project_members WHERE user_id = $1)`, userID).Scan(&exists)
	return exists, err
}

func (r *taskRepo) CreateTask(ctx context.Context, db DBTX, t *models.Task) error {
	query := `
        INSERT INTO tasks (project_id, title, description, comments, status, priority, due_at)
        VALUES ($1, $2, $3, NULL, $4, $5, $6)
        RETURNING id, created_at, updated_at;
    `
	return db.QueryRowContext(ctx, query, t.ProjectID, t.Title, t.Description, t.Status, t.Priority, t.DueAt).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *taskRepo) GetTaskByID(ctx context.Context, db DBTX, taskID int64) (*models.Task, error) {
	query := `SELECT id, project_id, title, description, comments, status, priority, due_at, created_at, updated_at FROM tasks WHERE id = $1`
	t, err := scanTask(db.QueryRowContext(ctx, query, taskID))
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *taskRepo) UpdateTaskDynamic(ctx context.Context, db DBTX, taskID int64, setParts []string, args []interface{}) (*models.Task, error) {
	setParts = append(setParts, "updated_at = NOW()")
	args = append(args, taskID)
	query := fmt.Sprintf(`
		UPDATE tasks
		SET %s
		WHERE id = $%d
		RETURNING id, project_id, title, description, comments, status, priority, due_at, created_at, updated_at;
	`, strings.Join(setParts, ", "), len(args))

	t, err := scanTask(db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *taskRepo) DeleteTask(ctx context.Context, db DBTX, taskID int64) error {
	_, err := db.ExecContext(ctx, `DELETE FROM tasks WHERE id = $1`, taskID)
	return err
}

func (r *taskRepo) AddAssignee(ctx context.Context, db DBTX, taskID, userID int64) error {
	_, err := db.ExecContext(ctx, `INSERT INTO task_assignees (task_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, taskID, userID)
	return err
}

func (r *taskRepo) ClearAssignees(ctx context.Context, db DBTX, taskID int64) error {
	_, err := db.ExecContext(ctx, `DELETE FROM task_assignees WHERE task_id = $1`, taskID)
	return err
}

func (r *taskRepo) GetAssigneesForTask(ctx context.Context, db DBTX, taskID int64) ([]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT user_id FROM task_assignees WHERE task_id = $1 ORDER BY user_id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}

func (r *taskRepo) GetAssigneesForTasks(ctx context.Context, db DBTX, taskIDs []int64) (map[int64][]int64, error) {
	if len(taskIDs) == 0 {
		return make(map[int64][]int64), nil
	}
	ph := makePlaceholders(1, len(taskIDs))
	args := make([]interface{}, len(taskIDs))
	for i, id := range taskIDs {
		args[i] = id
	}

	rows, err := db.QueryContext(ctx, `SELECT task_id, user_id FROM task_assignees WHERE task_id IN (`+ph+`) ORDER BY task_id, user_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64][]int64)
	for rows.Next() {
		var taskID, userID int64
		if err := rows.Scan(&taskID, &userID); err == nil {
			out[taskID] = append(out[taskID], userID)
		}
	}
	return out, rows.Err()
}

// Вспомогательная функция для извлечения массива задач из *sql.Rows
func fetchTasksList(rows *sql.Rows) ([]models.Task, []int64, error) {
	defer rows.Close()
	var tasks []models.Task
	var taskIDs []int64
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, nil, err
		}
		tasks = append(tasks, t)
		taskIDs = append(taskIDs, t.ID)
	}
	return tasks, taskIDs, rows.Err()
}

func (r *taskRepo) ListProjectTasks(ctx context.Context, db DBTX, projectID, limit, offset int64) ([]models.Task, []int64, error) {
	query := `
        SELECT id, project_id, title, description, comments, status, priority, due_at, created_at, updated_at
        FROM tasks WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3
    `
	rows, err := db.QueryContext(ctx, query, projectID, limit, offset)
	if err != nil {
		return nil, nil, err
	}
	return fetchTasksList(rows)
}

func (r *taskRepo) ListTasksWithFilter(ctx context.Context, db DBTX, userID int64, filter dto.TaskFilterRequest) ([]models.Task, []int64, error) {
	query := `
        SELECT DISTINCT t.id, t.project_id, t.title, t.description, t.comments, t.status, t.priority, t.due_at, t.created_at, t.updated_at
        FROM tasks t
        JOIN project_members pm ON pm.project_id = t.project_id
        WHERE pm.user_id = $1
    `
	args := []interface{}{userID}
	idx := 2

	if filter.ProjectID != nil {
		query += fmt.Sprintf(" AND t.project_id = $%d", idx)
		args = append(args, *filter.ProjectID)
		idx++
	}
	if filter.Priority != nil {
		query += fmt.Sprintf(" AND t.priority = $%d", idx)
		args = append(args, *filter.Priority)
	}
	query += " ORDER BY t.created_at DESC;"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	return fetchTasksList(rows)
}

func (r *taskRepo) ListTasksByAssignees(ctx context.Context, db DBTX, projectID int64, assignees []int64) ([]models.Task, []int64, error) {
	query := `
        SELECT id, project_id, title, description, comments, status, priority, due_at, created_at, updated_at
        FROM tasks WHERE project_id = $1
    `
	args := []interface{}{projectID}

	if len(assignees) > 0 {
		ph := makePlaceholders(2, len(assignees))
		query += fmt.Sprintf(` AND EXISTS (SELECT 1 FROM task_assignees ta WHERE ta.task_id = tasks.id AND ta.user_id IN (%s))`, ph)
		for _, uid := range assignees {
			args = append(args, uid)
		}
	}
	query += ` ORDER BY created_at DESC;`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	return fetchTasksList(rows)
}

func (r *taskRepo) ListTasksByCategory(ctx context.Context, db DBTX, projectID int64, category models.TaskCategory) ([]models.Task, []int64, error) {
	query := `
        SELECT id, project_id, title, description, comments, status, priority, due_at, created_at, updated_at
        FROM tasks WHERE project_id = $1
    `
	switch category {
	case models.TaskCategoryCurrent:
		query += ` AND status IN ('todo', 'in_progress', 'postponed') AND (due_at IS NULL OR due_at >= NOW())`
	case models.TaskCategoryReview:
		query += ` AND status = 'in_review'`
	case models.TaskCategoryInactive:
		query += ` AND (status = 'done' OR (status <> 'done' AND due_at IS NOT NULL AND due_at < NOW()))`
	}
	query += ` ORDER BY created_at DESC;`

	rows, err := db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, nil, err
	}
	return fetchTasksList(rows)
}

func (r *taskRepo) ListNearestTasks(ctx context.Context, db DBTX, userID int64) ([]models.Task, []int64, error) {
	query := `
        SELECT DISTINCT t.id, t.project_id, t.title, t.description, t.comments, t.status, t.priority, t.due_at, t.created_at, t.updated_at
        FROM tasks t
        JOIN task_assignees ta ON ta.task_id = t.id
        WHERE ta.user_id = $1 AND t.status <> 'done' AND t.due_at IS NOT NULL AND t.due_at >= NOW()
        ORDER BY t.due_at ASC, t.created_at ASC LIMIT 3;
    `
	rows, err := db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, nil, err
	}
	return fetchTasksList(rows)
}

func (r *taskRepo) GetMyTasks(ctx context.Context, db DBTX, userID int64) ([]models.Task, []int64, error) {
	query := `
        SELECT t.id, t.project_id, t.title, t.description, t.comments, t.status, t.priority, t.due_at, t.created_at, t.updated_at
        FROM tasks t
        JOIN task_assignees ta ON ta.task_id = t.id
        WHERE ta.user_id = $1 ORDER BY t.created_at DESC;
    `
	rows, err := db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, nil, err
	}
	return fetchTasksList(rows)
}

func (r *taskRepo) GetTaskProjectAndStatus(ctx context.Context, db DBTX, taskID int64) (int64, string, error) {
	var projectID int64
	var status string
	err := db.QueryRowContext(ctx, `SELECT project_id, status FROM tasks WHERE id = $1`, taskID).Scan(&projectID, &status)
	return projectID, status, err
}

func (r *taskRepo) UpdateTaskStatus(ctx context.Context, db DBTX, taskID int64, status string) error {
	_, err := db.ExecContext(ctx, `UPDATE tasks SET status = $1, updated_at = NOW() WHERE id = $2`, status, taskID)
	return err
}

func (r *taskRepo) SubmitForReview(ctx context.Context, db DBTX, taskID, projectID, userID int64) (int64, error) {
	q := `
    UPDATE tasks t
    SET status_before_review = t.status, status = 'in_review', submitted_at = NOW(), updated_at = NOW()
    FROM task_assignees ta
    WHERE t.id = $1 AND t.project_id = $2 AND t.status = 'in_progress' AND ta.task_id = t.id AND ta.user_id = $3
	`
	res, err := db.ExecContext(ctx, q, taskID, projectID, userID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *taskRepo) ReviewTaskApprove(ctx context.Context, db DBTX, taskID, projectID, ownerID int64, comment *string) (int64, error) {
	q := `
        UPDATE tasks SET status = 'done', reviewed_at = NOW(), reviewed_by = $1, review_comment = $2, updated_at = NOW()
        WHERE id = $3 AND project_id = $4 AND status = 'in_review'
    `
	res, err := db.ExecContext(ctx, q, ownerID, comment, taskID, projectID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *taskRepo) ReviewTaskReject(ctx context.Context, db DBTX, taskID, projectID, ownerID int64, comment *string, newDueAt *time.Time) (int64, error) {
	q := `
        UPDATE tasks SET status = 'in_progress', due_at = $1, reviewed_at = NOW(), reviewed_by = $2, review_comment = $3, updated_at = NOW()
        WHERE id = $4 AND project_id = $5 AND status = 'in_review'
    `
	res, err := db.ExecContext(ctx, q, newDueAt, ownerID, comment, taskID, projectID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
