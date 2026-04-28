package admin

import (
	stdcontext "context"
	"database/sql"
	"fmt"
	"time"
)

func ensureCRUDMenus(dbConn *sql.DB, tables []tableMeta) error {
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 10*time.Second)
	defer cancel()

	var dataMenuID int64
	err := dbConn.QueryRowContext(ctx, `
		SELECT id
		FROM goadmin_menu
		WHERE parent_id = 0
		  AND title = 'Data Management'
		LIMIT 1
	`).Scan(&dataMenuID)
	if err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("find data management menu: %w", err)
		}

		err = dbConn.QueryRowContext(ctx, `
			INSERT INTO goadmin_menu (parent_id, type, "order", title, icon, uri, header, plugin_name, created_at, updated_at)
			VALUES (0, 1, 100, 'Data Management', 'fa-database', '', NULL, '', NOW(), NOW())
			RETURNING id
		`).Scan(&dataMenuID)
		if err != nil {
			return fmt.Errorf("create data management menu: %w", err)
		}
	}

	for i, meta := range tables {
		if isPaymentPanelTable(meta.Name) {
			continue
		}

		uri := "/info/" + meta.Name
		title := toLabel(meta.Name)

		var menuID int64
		err = dbConn.QueryRowContext(ctx, `
			SELECT id
			FROM goadmin_menu
			WHERE uri = $1
			LIMIT 1
		`, uri).Scan(&menuID)
		if err != nil {
			if err != sql.ErrNoRows {
				return fmt.Errorf("find menu for %s: %w", meta.Name, err)
			}

			err = dbConn.QueryRowContext(ctx, `
				INSERT INTO goadmin_menu (parent_id, type, "order", title, icon, uri, header, plugin_name, created_at, updated_at)
				VALUES ($1, 1, $2, $3, 'fa-table', $4, NULL, '', NOW(), NOW())
				RETURNING id
			`, dataMenuID, i+1, title, uri).Scan(&menuID)
			if err != nil {
				return fmt.Errorf("create menu for %s: %w", meta.Name, err)
			}
		} else {
			if _, err := dbConn.ExecContext(ctx, `
				UPDATE goadmin_menu
				SET parent_id = $1,
				    title = $2,
				    "order" = $3,
				    updated_at = NOW()
				WHERE id = $4
			`, dataMenuID, title, i+1, menuID); err != nil {
				return fmt.Errorf("update menu for %s: %w", meta.Name, err)
			}
		}

		if _, err := dbConn.ExecContext(ctx, `
			INSERT INTO goadmin_role_menu (role_id, menu_id, created_at, updated_at)
			VALUES (1, $1, NOW(), NOW())
			ON CONFLICT (role_id, menu_id)
			DO UPDATE SET updated_at = NOW()
		`, menuID); err != nil {
			return fmt.Errorf("assign menu permission for %s: %w", meta.Name, err)
		}
	}

	if _, err := dbConn.ExecContext(ctx, `
		INSERT INTO goadmin_role_menu (role_id, menu_id, created_at, updated_at)
		VALUES (1, $1, NOW(), NOW())
		ON CONFLICT (role_id, menu_id)
		DO UPDATE SET updated_at = NOW()
	`, dataMenuID); err != nil {
		return fmt.Errorf("assign data management menu permission: %w", err)
	}

	return nil
}

func ensurePaymentAdminViews(dbConn *sql.DB) error {
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 10*time.Second)
	defer cancel()

	statements := []string{
		`CREATE OR REPLACE VIEW admin_payment_subscribers_v AS
		SELECT
			u.user_id,
			u.nama_lengkap,
			u.email,
			MAX(us.start_date) AS active_start_date,
			MAX(us.end_date) AS active_end_date,
			COUNT(DISTINCT us.user_subscription_id) AS total_subscriptions,
			COUNT(DISTINCT p.payment_id) FILTER (WHERE p.proof_document_id IS NOT NULL) AS total_proof_uploads,
			COUNT(DISTINCT p.payment_id) FILTER (WHERE p.status = 'success') AS total_success_payments
		FROM users u
		LEFT JOIN user_subscriptions us ON us.user_id = u.user_id
		LEFT JOIN payments p ON p.user_id = u.user_id
		GROUP BY u.user_id, u.nama_lengkap, u.email`,
		`CREATE OR REPLACE VIEW admin_payment_requests_v AS
		SELECT
			p.payment_id,
			p.transaction_id,
			p.user_id,
			u.nama_lengkap,
			u.email,
			p.package_name_snapshot,
			p.normal_price_snapshot,
			p.discount_price_snapshot,
			p.price_amount_snapshot,
			p.status,
			p.proof_document_id,
			p.created_at,
			p.expired_at
		FROM payments p
		JOIN users u ON u.user_id = p.user_id
		WHERE p.status IN ('pending', 'failed')`,
	}

	for _, stmt := range statements {
		if _, err := dbConn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure payment admin view: %w", err)
		}
	}

	return nil
}

func ensurePaymentAdminMenus(dbConn *sql.DB) error {
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 10*time.Second)
	defer cancel()

	mergeMenuInto := func(keepID, duplicateID int64) error {
		if keepID == 0 || duplicateID == 0 || keepID == duplicateID {
			return nil
		}

		if _, err := dbConn.ExecContext(ctx, `
			UPDATE goadmin_menu
			SET parent_id = $1,
			    updated_at = NOW()
			WHERE parent_id = $2
		`, keepID, duplicateID); err != nil {
			return fmt.Errorf("move duplicate menu children (%d -> %d): %w", duplicateID, keepID, err)
		}

		if _, err := dbConn.ExecContext(ctx, `
			INSERT INTO goadmin_role_menu (role_id, menu_id, created_at, updated_at)
			SELECT role_id, $1, NOW(), NOW()
			FROM goadmin_role_menu
			WHERE menu_id = $2
			ON CONFLICT (role_id, menu_id)
			DO UPDATE SET updated_at = NOW()
		`, keepID, duplicateID); err != nil {
			return fmt.Errorf("move duplicate role menu mapping (%d -> %d): %w", duplicateID, keepID, err)
		}

		if _, err := dbConn.ExecContext(ctx, `DELETE FROM goadmin_role_menu WHERE menu_id = $1`, duplicateID); err != nil {
			return fmt.Errorf("cleanup duplicate role menu mapping (%d): %w", duplicateID, err)
		}

		if _, err := dbConn.ExecContext(ctx, `DELETE FROM goadmin_menu WHERE id = $1`, duplicateID); err != nil {
			return fmt.Errorf("delete duplicate menu (%d): %w", duplicateID, err)
		}

		return nil
	}

	var (
		parentID       int64
		legacyParentID int64
	)

	err := dbConn.QueryRowContext(ctx, `
		SELECT id
		FROM goadmin_menu
		WHERE parent_id = 0
		  AND title = 'Payment Panel'
		LIMIT 1
	`).Scan(&parentID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("find payment panel menu: %w", err)
	}

	err = dbConn.QueryRowContext(ctx, `
		SELECT id
		FROM goadmin_menu
		WHERE parent_id = 0
		  AND title = 'Payment Admin'
		LIMIT 1
	`).Scan(&legacyParentID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("find legacy payment admin menu: %w", err)
	}

	if parentID == 0 && legacyParentID != 0 {
		if _, err := dbConn.ExecContext(ctx, `
			UPDATE goadmin_menu
			SET title = 'Payment Panel',
			    icon = 'fa-credit-card',
			    updated_at = NOW()
			WHERE id = $1
		`, legacyParentID); err != nil {
			return fmt.Errorf("rename legacy payment admin menu: %w", err)
		}
		parentID = legacyParentID
		legacyParentID = 0
	}

	if parentID == 0 {
		err = dbConn.QueryRowContext(ctx, `
			INSERT INTO goadmin_menu (parent_id, type, "order", title, icon, uri, header, plugin_name, created_at, updated_at)
			VALUES (0, 1, 90, 'Payment Panel', 'fa-credit-card', '', NULL, '', NOW(), NOW())
			RETURNING id
		`).Scan(&parentID)
		if err != nil {
			return fmt.Errorf("create payment panel menu: %w", err)
		}
	}

	if legacyParentID != 0 && legacyParentID != parentID {
		if err := mergeMenuInto(parentID, legacyParentID); err != nil {
			return fmt.Errorf("merge legacy payment admin menu: %w", err)
		}
	}

	parentRows, err := dbConn.QueryContext(ctx, `
		SELECT id
		FROM goadmin_menu
		WHERE parent_id = 0
		  AND title = 'Payment Panel'
		ORDER BY id ASC
	`)
	if err != nil {
		return fmt.Errorf("query duplicate payment panel parents: %w", err)
	}
	defer parentRows.Close()

	for parentRows.Next() {
		var duplicateParentID int64
		if err := parentRows.Scan(&duplicateParentID); err != nil {
			return fmt.Errorf("scan duplicate payment panel parent: %w", err)
		}
		if duplicateParentID == parentID {
			continue
		}
		if err := mergeMenuInto(parentID, duplicateParentID); err != nil {
			return fmt.Errorf("dedupe payment panel parent menu: %w", err)
		}
	}
	if err := parentRows.Err(); err != nil {
		return fmt.Errorf("iterate duplicate payment panel parents: %w", err)
	}

	if _, err := dbConn.ExecContext(ctx, `
		UPDATE goadmin_menu
		SET uri = '/payment-panel/status',
		    updated_at = NOW()
		WHERE uri = '/payment-admin/status'
	`); err != nil {
		return fmt.Errorf("normalize legacy payment status uri: %w", err)
	}

	items := []struct {
		Order int
		Title string
		URI   string
	}{
		{Order: 1, Title: "List Package", URI: "/info/subscriptions"},
		{Order: 2, Title: "List User Subscribe", URI: "/info/admin_payment_subscribers_v"},
		{Order: 3, Title: "User Request Payment", URI: "/info/admin_payment_requests_v"},
		{Order: 4, Title: "Ubah Status User", URI: "/payment-panel/status"},
	}

	allowedURIs := make(map[string]struct{}, len(items))
	for _, item := range items {
		allowedURIs[item.URI] = struct{}{}
	}

	childRows, err := dbConn.QueryContext(ctx, `
		SELECT id, uri
		FROM goadmin_menu
		WHERE parent_id = $1
	`, parentID)
	if err != nil {
		return fmt.Errorf("query payment panel children: %w", err)
	}

	var deleteChildIDs []int64
	for childRows.Next() {
		var (
			childID int64
			uri     string
		)
		if err := childRows.Scan(&childID, &uri); err != nil {
			childRows.Close()
			return fmt.Errorf("scan payment panel child: %w", err)
		}
		if _, ok := allowedURIs[uri]; !ok {
			deleteChildIDs = append(deleteChildIDs, childID)
		}
	}
	if err := childRows.Err(); err != nil {
		childRows.Close()
		return fmt.Errorf("iterate payment panel child: %w", err)
	}
	childRows.Close()

	for _, childID := range deleteChildIDs {
		if _, err := dbConn.ExecContext(ctx, `DELETE FROM goadmin_role_menu WHERE menu_id = $1`, childID); err != nil {
			return fmt.Errorf("delete legacy payment child role mapping (%d): %w", childID, err)
		}
		if _, err := dbConn.ExecContext(ctx, `DELETE FROM goadmin_menu WHERE id = $1`, childID); err != nil {
			return fmt.Errorf("delete legacy payment child menu (%d): %w", childID, err)
		}
	}

	for _, item := range items {
		dupRows, err := dbConn.QueryContext(ctx, `
			SELECT id
			FROM goadmin_menu
			WHERE uri = $1
			ORDER BY id ASC
		`, item.URI)
		if err != nil {
			return fmt.Errorf("query duplicate payment menu %s: %w", item.Title, err)
		}

		var dupIDs []int64
		for dupRows.Next() {
			var id int64
			if err := dupRows.Scan(&id); err != nil {
				dupRows.Close()
				return fmt.Errorf("scan duplicate payment menu %s: %w", item.Title, err)
			}
			dupIDs = append(dupIDs, id)
		}
		if err := dupRows.Err(); err != nil {
			dupRows.Close()
			return fmt.Errorf("iterate duplicate payment menu %s: %w", item.Title, err)
		}
		dupRows.Close()

		if len(dupIDs) > 1 {
			keepID := dupIDs[0]
			for _, duplicateID := range dupIDs[1:] {
				if err := mergeMenuInto(keepID, duplicateID); err != nil {
					return fmt.Errorf("dedupe payment menu %s: %w", item.Title, err)
				}
			}
		}

		var menuID int64
		err = dbConn.QueryRowContext(ctx, `
			SELECT id
			FROM goadmin_menu
			WHERE uri = $1
			LIMIT 1
		`, item.URI).Scan(&menuID)
		if err != nil {
			if err != sql.ErrNoRows {
				return fmt.Errorf("find payment menu %s: %w", item.Title, err)
			}

			err = dbConn.QueryRowContext(ctx, `
				INSERT INTO goadmin_menu (parent_id, type, "order", title, icon, uri, header, plugin_name, created_at, updated_at)
				VALUES ($1, 1, $2, $3, 'fa-circle-o', $4, NULL, '', NOW(), NOW())
				RETURNING id
			`, parentID, item.Order, item.Title, item.URI).Scan(&menuID)
			if err != nil {
				return fmt.Errorf("create payment menu %s: %w", item.Title, err)
			}
		} else {
			if _, err := dbConn.ExecContext(ctx, `
				UPDATE goadmin_menu
				SET parent_id = $1,
				    title = $2,
				    "order" = $3,
				    updated_at = NOW()
				WHERE id = $4
			`, parentID, item.Title, item.Order, menuID); err != nil {
				return fmt.Errorf("update payment menu %s: %w", item.Title, err)
			}
		}

		if _, err := dbConn.ExecContext(ctx, `
			INSERT INTO goadmin_role_menu (role_id, menu_id, created_at, updated_at)
			VALUES (1, $1, NOW(), NOW())
			ON CONFLICT (role_id, menu_id)
			DO UPDATE SET updated_at = NOW()
		`, menuID); err != nil {
			return fmt.Errorf("assign payment menu permission %s: %w", item.Title, err)
		}
	}

	if _, err := dbConn.ExecContext(ctx, `
		INSERT INTO goadmin_role_menu (role_id, menu_id, created_at, updated_at)
		VALUES (1, $1, NOW(), NOW())
		ON CONFLICT (role_id, menu_id)
		DO UPDATE SET updated_at = NOW()
	`, parentID); err != nil {
		return fmt.Errorf("assign payment parent menu permission: %w", err)
	}

	return nil
}

func ensureScholarshipHubMenus(dbConn *sql.DB) error {
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 10*time.Second)
	defer cancel()

	var tableExists bool
	if err := dbConn.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = 'funding_options'
		)
	`).Scan(&tableExists); err != nil {
		return fmt.Errorf("check funding_options table: %w", err)
	}

	if !tableExists {
		return nil
	}

	var parentID int64
	err := dbConn.QueryRowContext(ctx, `
		SELECT id
		FROM goadmin_menu
		WHERE parent_id = 0
		  AND title = 'Scholarship Hub Panel'
		LIMIT 1
	`).Scan(&parentID)
	if err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("find scholarship hub panel menu: %w", err)
		}

		err = dbConn.QueryRowContext(ctx, `
			INSERT INTO goadmin_menu (parent_id, type, "order", title, icon, uri, header, plugin_name, created_at, updated_at)
			VALUES (0, 1, 95, 'Scholarship Hub Panel', 'fa-graduation-cap', '', NULL, '', NOW(), NOW())
			RETURNING id
		`).Scan(&parentID)
		if err != nil {
			return fmt.Errorf("create scholarship hub panel menu: %w", err)
		}
	}

	var childID int64
	err = dbConn.QueryRowContext(ctx, `
		SELECT id
		FROM goadmin_menu
		WHERE uri = '/info/funding_options'
		LIMIT 1
	`).Scan(&childID)
	if err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("find scholarship hub child menu: %w", err)
		}

		err = dbConn.QueryRowContext(ctx, `
			INSERT INTO goadmin_menu (parent_id, type, "order", title, icon, uri, header, plugin_name, created_at, updated_at)
			VALUES ($1, 1, 1, 'Kelola Beasiswa', 'fa-circle-o', '/info/funding_options', NULL, '', NOW(), NOW())
			RETURNING id
		`, parentID).Scan(&childID)
		if err != nil {
			return fmt.Errorf("create scholarship hub child menu: %w", err)
		}
	} else {
		if _, err := dbConn.ExecContext(ctx, `
			UPDATE goadmin_menu
			SET parent_id = $1,
			    title = 'Kelola Beasiswa',
			    "order" = 1,
			    updated_at = NOW()
			WHERE id = $2
		`, parentID, childID); err != nil {
			return fmt.Errorf("update scholarship hub child menu: %w", err)
		}
	}

	for _, menuID := range []int64{parentID, childID} {
		if _, err := dbConn.ExecContext(ctx, `
			INSERT INTO goadmin_role_menu (role_id, menu_id, created_at, updated_at)
			VALUES (1, $1, NOW(), NOW())
			ON CONFLICT (role_id, menu_id)
			DO UPDATE SET updated_at = NOW()
		`, menuID); err != nil {
			return fmt.Errorf("assign scholarship hub menu permission: %w", err)
		}
	}

	return nil
}
