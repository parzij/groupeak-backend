package repository

import (
	"context"
	"database/sql"
	"groupeak/internal/models"
)

type NotificationRepository interface {
	Create(ctx context.Context, db DBTX, n *models.Notification) error
	GetUserNotifications(ctx context.Context, db DBTX, userID int64, limit, offset int) ([]models.Notification, error)
	MarkAsRead(ctx context.Context, db DBTX, userID int64, notificationIDs []int64) error
	MarkAllAsRead(ctx context.Context, db DBTX, userID int64) error
	GetUnreadCount(ctx context.Context, db DBTX, userID int64) (int, error)
}

type notificationRepo struct{}

func NewNotificationRepository() NotificationRepository {
	return &notificationRepo{}
}

func (r *notificationRepo) Create(ctx context.Context, db DBTX, n *models.Notification) error {
	query := `
		INSERT INTO notifications (project_id, user_id, actor_id, type, entity_id, payload, is_read)
		VALUES ($1, $2, $3, $4, $5, $6, false)
		RETURNING id, created_at;
	`
	return db.QueryRowContext(ctx, query, n.ProjectID, n.UserID, n.ActorID, n.Type, n.EntityID, n.Payload).Scan(&n.ID, &n.CreatedAt)
}

func (r *notificationRepo) GetUserNotifications(ctx context.Context, db DBTX, userID int64, limit, offset int) ([]models.Notification, error) {
	query := `
		SELECT id, project_id, user_id, actor_id, type, entity_id, payload, is_read, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3;
	`
	rows, err := db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifs []models.Notification
	for rows.Next() {
		var n models.Notification
		var actorID, entityID sql.NullInt64

		if err := rows.Scan(&n.ID, &n.ProjectID, &n.UserID, &actorID, &n.Type, &entityID, &n.Payload, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		if actorID.Valid {
			v := actorID.Int64
			n.ActorID = &v
		}
		if entityID.Valid {
			v := entityID.Int64
			n.EntityID = &v
		}
		notifs = append(notifs, n)
	}
	return notifs, nil
}

func (r *notificationRepo) MarkAsRead(ctx context.Context, db DBTX, userID int64, notificationIDs []int64) error {
	if len(notificationIDs) == 0 {
		return nil
	}
	_, err := db.ExecContext(ctx, `UPDATE notifications SET is_read = true WHERE user_id = $1 AND id = ANY($2)`, userID, notificationIDs)
	return err
}

func (r *notificationRepo) MarkAllAsRead(ctx context.Context, db DBTX, userID int64) error {
	_, err := db.ExecContext(ctx, `UPDATE notifications SET is_read = true WHERE user_id = $1 AND is_read = false`, userID)
	return err
}

func (r *notificationRepo) GetUnreadCount(ctx context.Context, db DBTX, userID int64) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false`, userID).Scan(&count)
	return count, err
}
