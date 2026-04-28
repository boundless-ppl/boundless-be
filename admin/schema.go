package admin

import (
	stdcontext "context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func ensureGoAdminSchema(dbConn *sql.DB) error {
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 10*time.Second)
	defer cancel()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS goadmin_users (
			id BIGSERIAL PRIMARY KEY,
			username VARCHAR(190) NOT NULL UNIQUE,
			password VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			avatar VARCHAR(255),
			remember_token VARCHAR(100),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS goadmin_roles (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(255) NOT NULL UNIQUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS goadmin_permissions (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(255) NOT NULL UNIQUE,
			http_method VARCHAR(255),
			http_path TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS goadmin_menu (
			id BIGSERIAL PRIMARY KEY,
			parent_id BIGINT NOT NULL DEFAULT 0,
			type INTEGER DEFAULT 1,
			"order" INTEGER NOT NULL DEFAULT 0,
			title VARCHAR(100) NOT NULL,
			icon VARCHAR(100) NOT NULL DEFAULT '',
			uri VARCHAR(255) NOT NULL DEFAULT '',
			header VARCHAR(255),
			uuid VARCHAR(100),
			plugin_name VARCHAR(150) NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS goadmin_operation_log (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			path VARCHAR(255) NOT NULL,
			method VARCHAR(20) NOT NULL,
			ip VARCHAR(64) NOT NULL,
			input TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS goadmin_session (
			id BIGSERIAL PRIMARY KEY,
			sid VARCHAR(100) NOT NULL UNIQUE,
			"values" TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS goadmin_role_users (
			role_id BIGINT NOT NULL,
			user_id BIGINT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (role_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS goadmin_role_permissions (
			role_id BIGINT NOT NULL,
			permission_id BIGINT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (role_id, permission_id)
		)`,
		`CREATE TABLE IF NOT EXISTS goadmin_role_menu (
			role_id BIGINT NOT NULL,
			menu_id BIGINT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (role_id, menu_id)
		)`,
		`CREATE TABLE IF NOT EXISTS goadmin_user_permissions (
			user_id BIGINT NOT NULL,
			permission_id BIGINT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (user_id, permission_id)
		)`,
		`CREATE TABLE IF NOT EXISTS goadmin_site (
			id BIGSERIAL PRIMARY KEY,
			key VARCHAR(100) NOT NULL UNIQUE,
			value TEXT NOT NULL,
			type INTEGER DEFAULT 0,
			description TEXT,
			state INTEGER DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`INSERT INTO goadmin_roles (id, name, slug)
			SELECT 1, 'Administrator', 'administrator'
			WHERE NOT EXISTS (SELECT 1 FROM goadmin_roles WHERE slug = 'administrator')`,
		`INSERT INTO goadmin_permissions (id, name, slug, http_method, http_path)
			SELECT 1, 'All permission', '*', '', '*'
			WHERE NOT EXISTS (SELECT 1 FROM goadmin_permissions WHERE slug = '*')`,
		`INSERT INTO goadmin_role_permissions (role_id, permission_id)
			SELECT 1, 1
			WHERE NOT EXISTS (SELECT 1 FROM goadmin_role_permissions WHERE role_id = 1 AND permission_id = 1)`,
		`INSERT INTO goadmin_menu (id, parent_id, type, "order", title, icon, uri, header, plugin_name)
			SELECT 1, 0, 1, 1, 'Dashboard', 'fa-bar-chart', '/', NULL, ''
			WHERE NOT EXISTS (SELECT 1 FROM goadmin_menu WHERE id = 1)`,
		`INSERT INTO goadmin_menu (id, parent_id, type, "order", title, icon, uri, header, plugin_name)
			SELECT 2, 0, 1, 2, 'Admin', 'fa-tasks', '', NULL, ''
			WHERE NOT EXISTS (SELECT 1 FROM goadmin_menu WHERE id = 2)`,
		`INSERT INTO goadmin_menu (id, parent_id, type, "order", title, icon, uri, header, plugin_name)
			SELECT 3, 2, 1, 1, 'Users', 'fa-users', '/info/manager', NULL, ''
			WHERE NOT EXISTS (SELECT 1 FROM goadmin_menu WHERE id = 3)`,
		`INSERT INTO goadmin_menu (id, parent_id, type, "order", title, icon, uri, header, plugin_name)
			SELECT 4, 2, 1, 2, 'Roles', 'fa-user', '/info/roles', NULL, ''
			WHERE NOT EXISTS (SELECT 1 FROM goadmin_menu WHERE id = 4)`,
		`INSERT INTO goadmin_menu (id, parent_id, type, "order", title, icon, uri, header, plugin_name)
			SELECT 5, 2, 1, 3, 'Permission', 'fa-ban', '/info/permission', NULL, ''
			WHERE NOT EXISTS (SELECT 1 FROM goadmin_menu WHERE id = 5)`,
		`INSERT INTO goadmin_menu (id, parent_id, type, "order", title, icon, uri, header, plugin_name)
			SELECT 6, 2, 1, 4, 'Menu', 'fa-bars', '/menu', NULL, ''
			WHERE NOT EXISTS (SELECT 1 FROM goadmin_menu WHERE id = 6)`,
		`INSERT INTO goadmin_menu (id, parent_id, type, "order", title, icon, uri, header, plugin_name)
			SELECT 7, 2, 1, 5, 'Operation log', 'fa-history', '/info/op', NULL, ''
			WHERE NOT EXISTS (SELECT 1 FROM goadmin_menu WHERE id = 7)`,
		`INSERT INTO goadmin_role_menu (role_id, menu_id)
			SELECT 1, 1 WHERE NOT EXISTS (SELECT 1 FROM goadmin_role_menu WHERE role_id = 1 AND menu_id = 1)`,
		`INSERT INTO goadmin_role_menu (role_id, menu_id)
			SELECT 1, 2 WHERE NOT EXISTS (SELECT 1 FROM goadmin_role_menu WHERE role_id = 1 AND menu_id = 2)`,
		`INSERT INTO goadmin_role_menu (role_id, menu_id)
			SELECT 1, 3 WHERE NOT EXISTS (SELECT 1 FROM goadmin_role_menu WHERE role_id = 1 AND menu_id = 3)`,
		`INSERT INTO goadmin_role_menu (role_id, menu_id)
			SELECT 1, 4 WHERE NOT EXISTS (SELECT 1 FROM goadmin_role_menu WHERE role_id = 1 AND menu_id = 4)`,
		`INSERT INTO goadmin_role_menu (role_id, menu_id)
			SELECT 1, 5 WHERE NOT EXISTS (SELECT 1 FROM goadmin_role_menu WHERE role_id = 1 AND menu_id = 5)`,
		`INSERT INTO goadmin_role_menu (role_id, menu_id)
			SELECT 1, 6 WHERE NOT EXISTS (SELECT 1 FROM goadmin_role_menu WHERE role_id = 1 AND menu_id = 6)`,
		`INSERT INTO goadmin_role_menu (role_id, menu_id)
			SELECT 1, 7 WHERE NOT EXISTS (SELECT 1 FROM goadmin_role_menu WHERE role_id = 1 AND menu_id = 7)`,
	}

	for _, stmt := range statements {
		if _, err := dbConn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure goadmin schema: %w", err)
		}
	}

	_, _ = dbConn.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('goadmin_users', 'id'), GREATEST((SELECT COALESCE(MAX(id), 1) FROM goadmin_users), 1), true)`)
	_, _ = dbConn.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('goadmin_roles', 'id'), GREATEST((SELECT COALESCE(MAX(id), 1) FROM goadmin_roles), 1), true)`)
	_, _ = dbConn.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('goadmin_permissions', 'id'), GREATEST((SELECT COALESCE(MAX(id), 1) FROM goadmin_permissions), 1), true)`)
	_, _ = dbConn.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('goadmin_menu', 'id'), GREATEST((SELECT COALESCE(MAX(id), 1) FROM goadmin_menu), 1), true)`)

	return nil
}

func ensureGoAdminSuperuser(dbConn *sql.DB, superuserEmail, superuserPassword string) error {
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 10*time.Second)
	defer cancel()

	superuserEmail = strings.ToLower(strings.TrimSpace(superuserEmail))
	if superuserEmail == "" || strings.TrimSpace(superuserPassword) == "" {
		return fmt.Errorf("superuser email and password are required")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(superuserPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash superuser password: %w", err)
	}

	var userID int64
	err = dbConn.QueryRowContext(ctx, `
		INSERT INTO goadmin_users (username, password, name, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (username)
		DO UPDATE SET
			password = EXCLUDED.password,
			name = EXCLUDED.name,
			updated_at = NOW()
		RETURNING id
	`, superuserEmail, string(hashedPassword), superuserEmail).Scan(&userID)
	if err != nil {
		return fmt.Errorf("upsert goadmin superuser: %w", err)
	}

	if _, err := dbConn.ExecContext(ctx, `
		INSERT INTO goadmin_role_users (role_id, user_id, created_at, updated_at)
		VALUES (1, $1, NOW(), NOW())
		ON CONFLICT (role_id, user_id)
		DO UPDATE SET updated_at = NOW()
	`, userID); err != nil {
		return fmt.Errorf("assign goadmin superuser role: %w", err)
	}

	if _, err := dbConn.ExecContext(ctx, `
		DELETE FROM goadmin_role_users
		WHERE user_id IN (
			SELECT id FROM goadmin_users WHERE username = 'admin' AND username <> $1
		)
	`, superuserEmail); err != nil {
		return fmt.Errorf("cleanup legacy goadmin admin role mapping: %w", err)
	}

	if _, err := dbConn.ExecContext(ctx, `
		DELETE FROM goadmin_users
		WHERE username = 'admin' AND username <> $1
	`, superuserEmail); err != nil {
		return fmt.Errorf("cleanup legacy goadmin admin user: %w", err)
	}

	return nil
}

func ensureAppAdminUser(dbConn *sql.DB, superuserEmail, superuserPassword, superuserName string) error {
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 10*time.Second)
	defer cancel()

	email := strings.ToLower(strings.TrimSpace(superuserEmail))
	password := strings.TrimSpace(superuserPassword)
	name := strings.TrimSpace(superuserName)
	if email == "" || password == "" {
		return fmt.Errorf("superuser email and password are required")
	}
	if name == "" {
		name = email
	}

	var usersTableExists bool
	if err := dbConn.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = 'users'
		)
	`).Scan(&usersTableExists); err != nil {
		return fmt.Errorf("check users table existence: %w", err)
	}
	if !usersTableExists {
		return fmt.Errorf("users table does not exist; run database migrations first")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash superuser password for app user: %w", err)
	}

	var existing struct {
		UserID string
		Role   string
	}
	err = dbConn.QueryRowContext(ctx, `
		SELECT user_id, role
		FROM users
		WHERE lower(email) = lower($1)
		LIMIT 1
	`, email).Scan(&existing.UserID, &existing.Role)
	if err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("find app superuser by email: %w", err)
		}

		if _, err := dbConn.ExecContext(ctx, `
			INSERT INTO users (
				user_id, nama_lengkap, role, email, password_hash, created_at,
				failed_login_count, first_failed_at, locked_until
			)
			VALUES ($1, $2, 'admin', $3, $4, NOW(), 0, NULL, NULL)
		`, uuid.NewString(), name, email, string(hashedPassword)); err != nil {
			return fmt.Errorf("create app superuser: %w", err)
		}

		return nil
	}

	if !strings.EqualFold(strings.TrimSpace(existing.Role), "admin") {
		return fmt.Errorf("superuser email %s exists with non-admin role; promote this account to admin first", email)
	}

	if _, err := dbConn.ExecContext(ctx, `
		UPDATE users
		SET password_hash = $1,
		    failed_login_count = 0,
		    first_failed_at = NULL,
		    locked_until = NULL
		WHERE user_id = $2
	`, string(hashedPassword), existing.UserID); err != nil {
		return fmt.Errorf("sync app superuser password: %w", err)
	}

	return nil
}
