package workers

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"groupeak/internal/eventbus"
	"groupeak/internal/models"
)

type DeadlineWorker struct {
	db     *sql.DB
	bus    *eventbus.EventBus
	logger *slog.Logger
}

func NewDeadlineWorker(db *sql.DB, bus *eventbus.EventBus, logger *slog.Logger) *DeadlineWorker {
	return &DeadlineWorker{
		db:     db,
		bus:    bus,
		logger: logger,
	}
}

func (w *DeadlineWorker) Start(ctx context.Context) {
	w.logger.Info("starting deadline worker (runs every 15m)")

	ticker := time.NewTicker(15 * time.Minute)

	go func() {
		for {
			select {
			case <-ctx.Done():
				w.logger.Info("shutting down deadline worker")
				ticker.Stop()
				return
			case <-ticker.C:
				w.checkTasks(ctx)
				w.checkEvents(ctx)
			}
		}
	}()
}

func (w *DeadlineWorker) checkTasks(ctx context.Context) {
	now := time.Now()
	windowStart := now.Add(23*time.Hour + 45*time.Minute)
	windowEnd := now.Add(24 * time.Hour)

	query := `
		SELECT id, project_id, title 
		FROM tasks 
		WHERE status IN ('todo', 'in_progress') 
		  AND due_at >= $1 AND due_at < $2;
	`
	rows, err := w.db.QueryContext(ctx, query, windowStart, windowEnd)
	if err != nil {
		w.logger.Error("deadline worker: failed to query tasks", slog.String("error", err.Error()))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, projectID int64
		var title string
		if err := rows.Scan(&id, &projectID, &title); err != nil {
			continue
		}

		w.bus.Publish(eventbus.SystemEvent{
			Type:      models.EventTypeTaskDeadline,
			ProjectID: projectID,
			ActorID:   0,
			EntityID:  id,
			Payload: map[string]interface{}{
				"title":   title,
				"message": "До дедлайна задачи осталось менее 24 часов!",
			},
		})
	}
}

func (w *DeadlineWorker) checkEvents(ctx context.Context) {
	now := time.Now()
	windowStart := now.Add(23*time.Hour + 45*time.Minute)
	windowEnd := now.Add(24 * time.Hour)

	query := `
		SELECT id, project_id, title 
		FROM events 
		WHERE start_at >= $1 AND start_at < $2;
	`
	rows, err := w.db.QueryContext(ctx, query, windowStart, windowEnd)
	if err != nil {
		w.logger.Error("deadline worker: failed to query events", slog.String("error", err.Error()))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, projectID int64
		var title string
		if err := rows.Scan(&id, &projectID, &title); err != nil {
			continue
		}

		w.bus.Publish(eventbus.SystemEvent{
			Type:      models.EventTypeEventReminder,
			ProjectID: projectID,
			ActorID:   0,
			EntityID:  id,
			Payload: map[string]interface{}{
				"title":   title,
				"message": "Созвон начнется через 24 часа!",
			},
		})
	}
}
