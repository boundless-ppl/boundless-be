package admin

import (
	stdcontext "context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	goadminctx "github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/auth"
	goadminauth "github.com/GoAdminGroup/go-admin/modules/auth"
	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func resolveGoAdminAppUserID(ctx *gin.Context, appDB *sql.DB) (string, bool) {
	cookie, err := ctx.Request.Cookie(goadminauth.DefaultCookieKey)
	if err != nil || cookie.Value == "" {
		return "", false
	}

	var sessionValuesRaw string
	err = appDB.QueryRowContext(ctx.Request.Context(), `
		SELECT "values"
		FROM goadmin_session
		WHERE sid = $1
	`, cookie.Value).Scan(&sessionValuesRaw)
	if err != nil {
		return "", false
	}

	var sessionValues map[string]interface{}
	if err := json.Unmarshal([]byte(sessionValuesRaw), &sessionValues); err != nil {
		return "", false
	}

	rawUserID, ok := sessionValues["user_id"]
	if !ok {
		return "", false
	}

	var goAdminUserID int64
	switch value := rawUserID.(type) {
	case float64:
		goAdminUserID = int64(value)
	case int64:
		goAdminUserID = value
	case int:
		goAdminUserID = int64(value)
	default:
		return "", false
	}

	var email string
	if err := appDB.QueryRowContext(ctx.Request.Context(), `SELECT username FROM goadmin_users WHERE id = $1`, goAdminUserID).Scan(&email); err != nil {
		return "", false
	}

	var appUserID string
	if err := appDB.QueryRowContext(ctx.Request.Context(), `
		SELECT user_id
		FROM users
		WHERE lower(email) = lower($1)
		  AND lower(role) = 'admin'
		LIMIT 1
	`, email).Scan(&appUserID); err != nil {
		return "", false
	}

	return appUserID, true
}

func appLoginProcessor(dbConn *sql.DB) auth.Processor {
	return func(ctx *goadminctx.Context) (models.UserModel, bool, string) {
		email := strings.ToLower(strings.TrimSpace(ctx.FormValue("username")))
		password := ctx.FormValue("password")
		if email == "" || password == "" {
			return models.UserModel{}, false, "wrong password or username"
		}

		var (
			fullName     string
			role         string
			userEmail    string
			passwordHash string
		)

		row := dbConn.QueryRowContext(ctx.Request.Context(), `
			SELECT nama_lengkap, role, email, password_hash
			FROM users
			WHERE lower(email) = $1
			  AND lower(role) = 'admin'
			LIMIT 1
		`, email)

		if err := row.Scan(&fullName, &role, &userEmail, &passwordHash); err != nil {
			return models.UserModel{}, false, "wrong password or username"
		}

		if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
			return models.UserModel{}, false, "wrong password or username"
		}

		adminID, err := upsertGoAdminUser(ctx.Request.Context(), dbConn, strings.ToLower(strings.TrimSpace(userEmail)), fullName, passwordHash)
		if err != nil {
			return models.UserModel{}, false, "login failed"
		}

		user := models.UserModel{
			Id:       adminID,
			Name:     fullName,
			UserName: strings.ToLower(strings.TrimSpace(userEmail)),
			Password: passwordHash,
			Roles: []models.RoleModel{{
				Id:   1,
				Name: "Administrator",
				Slug: "administrator",
			}},
			Permissions: []models.PermissionModel{{
				Id:         1,
				Name:       "All permission",
				Slug:       "*",
				HttpMethod: []string{""},
				HttpPath:   []string{"*"},
			}},
			Level:     role,
			LevelName: "Administrator",
		}

		return user, true, "ok"
	}
}

func upsertGoAdminUser(ctx stdcontext.Context, dbConn *sql.DB, email, fullName, passwordHash string) (int64, error) {
	var adminID int64
	err := dbConn.QueryRowContext(ctx, `
		INSERT INTO goadmin_users (username, password, name, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (username)
		DO UPDATE SET
			password = EXCLUDED.password,
			name = EXCLUDED.name,
			updated_at = NOW()
		RETURNING id
	`, email, passwordHash, fullName).Scan(&adminID)
	if err != nil {
		return 0, fmt.Errorf("upsert goadmin user: %w", err)
	}

	if _, err := dbConn.ExecContext(ctx, `
		INSERT INTO goadmin_role_users (role_id, user_id, created_at, updated_at)
		VALUES (1, $1, NOW(), NOW())
		ON CONFLICT (role_id, user_id)
		DO UPDATE SET updated_at = NOW()
	`, adminID); err != nil {
		return 0, fmt.Errorf("upsert goadmin role user: %w", err)
	}

	return adminID, nil
}
