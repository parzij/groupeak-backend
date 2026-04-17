package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"groupeak/internal/models"
)

type EventRepository interface {
	IsEventAuthorized(ctx context.Context, db DBTX, userID, projectID, eventID int) (bool, error)
	CheckUsersExist(ctx context.Context, db DBTX, userIDs []int) (bool, error)

	CreateEvent(ctx context.Context, db DBTX, e *models.Event) error
	UpdateEventDynamic(ctx context.Context, db DBTX, eventID int, setParts []string, args []interface{}) error
	DeleteEvent(ctx context.Context, db DBTX, eventID int) error

	AddParticipants(ctx context.Context, db DBTX, eventID int, userIDs []int) error
	ClearParticipants(ctx context.Context, db DBTX, eventID int) error
	GetParticipantsForEvents(ctx context.Context, db DBTX, eventIDs []int) (map[int][]int, error)

	GetUserEvents(ctx context.Context, db DBTX, userID int) ([]models.Event, []int, error)
}

type eventRepo struct{}

func NewEventRepository() EventRepository {
	return &eventRepo{}
}

func (r *eventRepo) CheckUsersExist(ctx context.Context, db DBTX, userIDs []int) (bool, error) {
	if len(userIDs) == 0 {
		return true, nil
	}

	unique := make(map[int]struct{})
	for _, id := range userIDs {
		unique[id] = struct{}{}
	}

	var b strings.Builder
	args := make([]interface{}, 0, len(unique))
	i := 1
	for id := range unique {
		if i > 1 {
			b.WriteByte(',')
		}
		b.WriteByte('$')
		b.WriteString(strconv.Itoa(i))
		args = append(args, id)
		i++
	}

	query := `SELECT COUNT(id) FROM users WHERE id IN (` + b.String() + `)`
	var count int
	err := db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == len(unique), nil
}

func (r *eventRepo) IsEventAuthorized(ctx context.Context, db DBTX, userID, projectID, eventID int) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM event_participants ep
			WHERE ep.event_id = $1 AND ep.user_id = $2
		) OR (
			SELECT owner_id FROM projects WHERE id = $3
		) = $2
	`
	var authorized bool
	err := db.QueryRowContext(ctx, query, eventID, userID, projectID).Scan(&authorized)
	return authorized, err
}

func (r *eventRepo) CreateEvent(ctx context.Context, db DBTX, e *models.Event) error {
	query := `
		INSERT INTO project_events (project_id, title, meeting_url, description, start_at, end_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	return db.QueryRowContext(ctx, query, e.ProjectID, e.Title, e.MeetingURL, e.Description, e.StartAt, e.EndAt).
		Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

func (r *eventRepo) UpdateEventDynamic(ctx context.Context, db DBTX, eventID int, setParts []string, args []interface{}) error {
	setParts = append(setParts, "updated_at = NOW()")
	args = append(args, eventID)

	query := fmt.Sprintf(`
		UPDATE project_events
		SET %s
		WHERE id = $%d
	`, strings.Join(setParts, ", "), len(args))

	_, err := db.ExecContext(ctx, query, args...)
	return err
}

func (r *eventRepo) DeleteEvent(ctx context.Context, db DBTX, eventID int) error {
	res, err := db.ExecContext(ctx, `DELETE FROM project_events WHERE id = $1`, eventID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *eventRepo) AddParticipants(ctx context.Context, db DBTX, eventID int, userIDs []int) error {
	if len(userIDs) == 0 {
		return nil
	}

	valueStrings := make([]string, 0, len(userIDs))
	valueArgs := make([]interface{}, 0, len(userIDs)*2)

	for i, uid := range userIDs {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2))
		valueArgs = append(valueArgs, eventID, uid)
	}

	query := fmt.Sprintf("INSERT INTO event_participants (event_id, user_id) VALUES %s ON CONFLICT DO NOTHING", strings.Join(valueStrings, ","))
	_, err := db.ExecContext(ctx, query, valueArgs...)
	return err
}

func (r *eventRepo) ClearParticipants(ctx context.Context, db DBTX, eventID int) error {
	_, err := db.ExecContext(ctx, `DELETE FROM event_participants WHERE event_id = $1`, eventID)
	return err
}

func (r *eventRepo) GetParticipantsForEvents(ctx context.Context, db DBTX, eventIDs []int) (map[int][]int, error) {
	if len(eventIDs) == 0 {
		return make(map[int][]int), nil
	}

	var b strings.Builder
	args := make([]interface{}, len(eventIDs))
	for i, id := range eventIDs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('$')
		b.WriteString(strconv.Itoa(i + 1))
		args[i] = id
	}

	query := `SELECT event_id, user_id FROM event_participants WHERE event_id IN (` + b.String() + `) ORDER BY event_id, user_id`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int][]int)
	for rows.Next() {
		var eID, uID int
		if err := rows.Scan(&eID, &uID); err == nil {
			out[eID] = append(out[eID], uID)
		}
	}
	return out, rows.Err()
}

func (r *eventRepo) GetUserEvents(ctx context.Context, db DBTX, userID int) ([]models.Event, []int, error) {
	query := `
		SELECT e.id, e.project_id, e.title, e.meeting_url, e.description, e.start_at, e.end_at, e.created_at, e.updated_at
		FROM project_events e
		JOIN event_participants ep ON e.id = ep.event_id
		WHERE ep.user_id = $1
		ORDER BY e.start_at ASC
	`
	rows, err := db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var events []models.Event
	var eventIDs []int
	for rows.Next() {
		var e models.Event
		err := rows.Scan(
			&e.ID, &e.ProjectID, &e.Title, &e.MeetingURL, &e.Description,
			&e.StartAt, &e.EndAt, &e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			return nil, nil, err
		}
		events = append(events, e)
		eventIDs = append(eventIDs, e.ID)
	}
	return events, eventIDs, rows.Err()
}
