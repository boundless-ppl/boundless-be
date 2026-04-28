package admin

import (
	"database/sql"
	"fmt"
	"net/mail"
	"os"
	"strings"
	"time"

	"boundless-be/model"
	"boundless-be/repository"
	boundlesspayment "boundless-be/service"

	_ "github.com/GoAdminGroup/go-admin/adapter/gin"
	goadminctx "github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/engine"
	"github.com/GoAdminGroup/go-admin/modules/config"
	_ "github.com/GoAdminGroup/go-admin/modules/db/drivers/postgres"
	"github.com/GoAdminGroup/go-admin/modules/language"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	goadmintemplate "github.com/GoAdminGroup/go-admin/template"
	_ "github.com/GoAdminGroup/themes/adminlte"
	"github.com/gin-gonic/gin"
)

func Setup(router *gin.Engine, databaseURL string, appDB *sql.DB) (*engine.Engine, error) {
	if appDB == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	superuserEmail, superuserPassword, err := loadSuperuserCredentials(os.Getenv)
	if err != nil {
		return nil, err
	}

	goadmintemplate.AddLoginComp(newEmailLoginComponent())

	language.AppendTo(language.EN, map[string]string{"username": "Email"})
	language.AppendTo(language.CN, map[string]string{"username": "Email"})
	language.AppendTo(language.JP, map[string]string{"username": "Email"})
	language.AppendTo(language.TC, map[string]string{"username": "Email"})
	language.AppendTo(language.PTBR, map[string]string{"username": "Email"})

	if err := ensureGoAdminSchema(appDB); err != nil {
		return nil, err
	}
	if err := ensureAppAdminUser(appDB, superuserEmail, superuserPassword, strings.TrimSpace(os.Getenv("SUPERUSER_NAME"))); err != nil {
		return nil, err
	}
	if err := ensureGoAdminSuperuser(appDB, superuserEmail, superuserPassword); err != nil {
		return nil, err
	}
	if err := ensurePaymentAdminViews(appDB); err != nil {
		return nil, err
	}

	tables, err := discoverTables(appDB)
	if err != nil {
		return nil, err
	}
	if err := ensureCRUDMenus(appDB, tables); err != nil {
		return nil, err
	}
	if err := ensurePaymentAdminMenus(appDB); err != nil {
		return nil, err
	}
	if err := ensureScholarshipHubMenus(appDB); err != nil {
		return nil, err
	}

	paymentService := boundlesspayment.NewPaymentService(repository.NewPaymentRepository(appDB))
	router.GET("/"+urlPrefix+"/payment-panel/status", func(ctx *gin.Context) {
		handlePaymentStatusPanel(ctx, appDB, paymentService)
	})
	router.POST("/"+urlPrefix+"/payment-panel/status", func(ctx *gin.Context) {
		handlePaymentStatusUpdate(ctx, appDB, paymentService)
	})

	generators := make(table.GeneratorList, len(tables))
	for _, t := range tables {
		meta := t
		generators[meta.Name] = func(ctx *goadminctx.Context) table.Table {
			return buildTable(ctx, meta, appDB)
		}
	}

	indexURL := "/info/manager"
	if len(tables) > 0 {
		indexURL = "/info/" + tables[0].Name
		for _, t := range tables {
			if t.Name == "users" {
				indexURL = "/info/users"
				break
			}
		}
	}

	router.GET("/"+urlPrefix, func(ctx *gin.Context) {
		ctx.Redirect(302, "/"+urlPrefix+indexURL)
	})
	router.GET("/"+urlPrefix+"/", func(ctx *gin.Context) {
		ctx.Redirect(302, "/"+urlPrefix+indexURL)
	})

	eng := engine.Default()
	cfg := config.Config{
		Env: config.EnvLocal,
		Databases: config.DatabaseList{
			"default": {
				Driver:          driverName,
				Dsn:             databaseURL,
				MaxIdleConns:    10,
				MaxOpenConns:    20,
				ConnMaxLifetime: 30 * time.Minute,
			},
		},
		UrlPrefix:          urlPrefix,
		IndexUrl:           indexURL,
		Debug:              false,
		AccessLogOff:       true,
		InfoLogOff:         true,
		ErrorLogOff:        true,
		AccessAssetsLogOff: true,
	}

	if err := eng.AddConfig(&cfg).
		AddAuthService(appLoginProcessor(appDB)).
		AddGenerators(generators).
		Use(router); err != nil {
		return nil, err
	}

	return eng, nil
}

func loadSuperuserCredentials(getenv func(string) string) (string, string, error) {
	superuserEmail := strings.ToLower(strings.TrimSpace(getenv("SUPERUSER_EMAIL")))
	superuserPassword := strings.TrimSpace(getenv("SUPERUSER_PASSWORD"))

	if superuserEmail == "" || superuserPassword == "" {
		return "", "", fmt.Errorf("SUPERUSER_EMAIL and SUPERUSER_PASSWORD environment variables are required")
	}

	if superuserEmail == "admin" && superuserPassword == "admin" {
		return "", "", fmt.Errorf("SUPERUSER credentials cannot use insecure default admin/admin")
	}

	if _, err := mail.ParseAddress(superuserEmail); err != nil {
		return "", "", fmt.Errorf("SUPERUSER_EMAIL must be a valid email address")
	}

	if len(superuserPassword) < 12 || !model.IsPasswordComplex(superuserPassword) {
		return "", "", fmt.Errorf("SUPERUSER_PASSWORD must be at least 12 chars and contain upper, lower, number, and special char")
	}

	return superuserEmail, superuserPassword, nil
}
