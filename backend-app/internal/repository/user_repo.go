package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"groupeak/internal/models"
)

type UserRepository interface {
	CheckEmailExists(ctx context.Context, db DBTX, email string) (bool, error)
	CreateUser(ctx context.Context, db DBTX, user *models.User, passwordHash string) error
	GetUserByEmail(ctx context.Context, db DBTX, email string) (*models.User, string, error)
	GetUserByID(ctx context.Context, db DBTX, userID int64) (*models.User, string, error)
	UpdatePassword(ctx context.Context, db DBTX, userID int64, newHash string) error
	UpdateEmail(ctx context.Context, db DBTX, userID int64, newEmail string) error
	UpdateProfileDynamic(ctx context.Context, db DBTX, userID int64, setParts []string, args []interface{}) (*models.User, error)
	GetTaskStats(ctx context.Context, db DBTX, userID int64) (models.TaskStats, error)
	UpdateAvatarURL(ctx context.Context, db DBTX, userID int64, avatarURL *string) error
}

type userRepo struct{}

func NewUserRepository() UserRepository {
	return &userRepo{}
}

func (r *userRepo) CheckEmailExists(ctx context.Context, db DBTX, email string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	return exists, err
}

func (r *userRepo) CreateUser(ctx context.Context, db DBTX, u *models.User, passwordHash string) error {
	query := `
		INSERT INTO users (full_name, email, password_hash, position, avatar_url, birth_date, about)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at;
	`
	var birthDate interface{}
	if u.BirthDate != nil {
		birthDate = *u.BirthDate
	}
	return db.QueryRowContext(ctx, query,
		u.FullName, u.Email, passwordHash, u.Position, u.AvatarURL, birthDate, u.About,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

func (r *userRepo) GetUserByEmail(ctx context.Context, db DBTX, email string) (*models.User, string, error) {
	var u models.User
	var hash string
	var avatar, position, about sql.NullString
	var birthDate sql.NullTime

	query := `SELECT id, full_name, email, password_hash, position, avatar_url, birth_date, about, created_at, updated_at FROM users WHERE email = $1`
	err := db.QueryRowContext(ctx, query, email).Scan(
		&u.ID, &u.FullName, &u.Email, &hash, &position, &avatar, &birthDate, &about, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, "", err
	}

	if avatar.Valid {
		u.AvatarURL = &avatar.String
	}
	if position.Valid {
		u.Position = &position.String
	}
	if about.Valid {
		u.About = &about.String
	}
	if birthDate.Valid {
		bd := birthDate.Time.Format("02.01.2006")
		u.BirthDate = &bd
	}

	return &u, hash, nil
}

func (r *userRepo) GetUserByID(ctx context.Context, db DBTX, userID int64) (*models.User, string, error) {
	var u models.User
	var hash string
	var avatar, position, about sql.NullString
	var birthDate sql.NullTime

	query := `SELECT id, full_name, email, password_hash, position, avatar_url, birth_date, about, created_at, updated_at FROM users WHERE id = $1`
	err := db.QueryRowContext(ctx, query, userID).Scan(
		&u.ID, &u.FullName, &u.Email, &hash, &position, &avatar, &birthDate, &about, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, "", err
	}

	if avatar.Valid {
		u.AvatarURL = &avatar.String
	}
	if position.Valid {
		u.Position = &position.String
	}
	if about.Valid {
		u.About = &about.String
	}
	if birthDate.Valid {
		bd := birthDate.Time.Format("02.01.2006")
		u.BirthDate = &bd
	}

	return &u, hash, nil
}

func (r *userRepo) UpdatePassword(ctx context.Context, db DBTX, userID int64, newHash string) error {
	_, err := db.ExecContext(ctx, `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`, newHash, userID)
	return err
}

func (r *userRepo) UpdateEmail(ctx context.Context, db DBTX, userID int64, newEmail string) error {
	_, err := db.ExecContext(ctx, `UPDATE users SET email = $1, updated_at = NOW() WHERE id = $2`, newEmail, userID)
	return err
}

func (r *userRepo) UpdateProfileDynamic(ctx context.Context, db DBTX, userID int64, setParts []string, args []interface{}) (*models.User, error) {
	setParts = append(setParts, "updated_at = NOW()")
	args = append(args, userID)

	query := fmt.Sprintf(`
		UPDATE users
		SET %s
		WHERE id = $%d
		RETURNING id, full_name, email, position, avatar_url, birth_date, about, created_at, updated_at;
	`, strings.Join(setParts, ", "), len(args))

	var u models.User
	var avatar, position, about sql.NullString
	var birthDate sql.NullTime

	err := db.QueryRowContext(ctx, query, args...).Scan(
		&u.ID, &u.FullName, &u.Email, &position, &avatar, &birthDate, &about, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if avatar.Valid {
		u.AvatarURL = &avatar.String
	}
	if position.Valid {
		u.Position = &position.String
	}
	if about.Valid {
		u.About = &about.String
	}
	if birthDate.Valid {
		bd := birthDate.Time.Format("02.01.2006")
		u.BirthDate = &bd
	}

	return &u, nil
}

func (r *userRepo) GetTaskStats(ctx context.Context, db DBTX, userID int64) (models.TaskStats, error) {
	query := `
        SELECT
            COUNT(*) FILTER (WHERE t.status = 'done') AS done,
            COUNT(*) FILTER (
                WHERE t.status <> 'done'
                  AND t.due_at IS NOT NULL
                  AND t.due_at < NOW()
            ) AS overdue
        FROM tasks t
        JOIN task_assignees ta ON ta.task_id = t.id
        WHERE ta.user_id = $1
    `
	var st models.TaskStats
	err := db.QueryRowContext(ctx, query, userID).Scan(&st.Done, &st.Overdue)
	return st, err
}

func (r *userRepo) UpdateAvatarURL(ctx context.Context, db DBTX, userID int64, avatarURL *string) error {
	res, err := db.ExecContext(ctx, `UPDATE users SET avatar_url = $1, updated_at = NOW() WHERE id = $2`, avatarURL, userID)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
