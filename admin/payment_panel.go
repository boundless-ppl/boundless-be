package admin

import (
	stdcontext "context"
	"database/sql"
	"fmt"
	htmltemplate "html/template"
	"log"
	"strings"
	"time"

	"boundless-be/model"
	boundlesspayment "boundless-be/service"

	"github.com/gin-gonic/gin"
)

type paymentStatusPanelItem struct {
	PaymentID        string
	TransactionID    string
	UserName         string
	UserEmail        string
	PackageName      string
	Amount           int64
	Status           string
	TransactionAt    string
	ExpiredAt        string
	ExpiryStatus     string
	IsExpired        bool
	ProofDocumentID  *string
	ProofDocumentURL *string
}

func handlePaymentStatusPanel(ctx *gin.Context, appDB *sql.DB, paymentService *boundlesspayment.PaymentService) {
	applyAdminSecurityHeaders(ctx)

	if _, ok := resolveGoAdminAppUserID(ctx, appDB); !ok {
		ctx.Redirect(302, "/"+urlPrefix+"/login")
		return
	}

	csrfToken, err := generateCSRFToken()
	if err != nil {
		log.Printf("admin payment panel csrf generation failed: %v", err)
		ctx.String(500, "failed to load payment status panel")
		return
	}
	setAdminCSRFCookie(ctx, csrfToken)

	page := 1
	pageSize := 50

	result, err := paymentService.ListAdminPayments(ctx.Request.Context(), "", string(model.PaymentStatusPending), page, pageSize)
	if err != nil {
		log.Printf("admin payment panel load failed: %v", err)
		ctx.String(500, "failed to load payment status panel")
		return
	}

	items := make([]paymentStatusPanelItem, 0, len(result.Items))
	for _, row := range result.Items {
		expiredAtLabel := "-"
		expiryStatus := "Belum ada batas expiry"
		isExpired := false
		if !row.ExpiredAt.IsZero() {
			expiredAtUTC := row.ExpiredAt.UTC()
			expiredAtLabel = expiredAtUTC.Format("2006-01-02 15:04:05")
			isExpired = time.Now().UTC().After(expiredAtUTC)
			if isExpired {
				expiryStatus = "Sudah Expired"
			} else {
				expiryStatus = "Belum Expired"
			}
		}

		items = append(items, paymentStatusPanelItem{
			PaymentID:        row.PaymentID,
			TransactionID:    row.TransactionID,
			UserName:         row.UserName,
			UserEmail:        row.UserEmail,
			PackageName:      row.PackageName,
			Amount:           row.Amount,
			Status:           string(row.Status),
			TransactionAt:    row.TransactionAt.UTC().Format(time.RFC3339),
			ExpiredAt:        expiredAtLabel,
			ExpiryStatus:     expiryStatus,
			IsExpired:        isExpired,
			ProofDocumentID:  row.ProofDocumentID,
			ProofDocumentURL: row.ProofDocumentURL,
		})
	}

	pageData := struct {
		Title  string
		Items  []paymentStatusPanelItem
		Prefix string
		CSRF   string
	}{
		Title:  "Payment Status Panel",
		Items:  items,
		Prefix: "/" + urlPrefix,
		CSRF:   csrfToken,
	}

	tmpl := htmltemplate.Must(htmltemplate.New("payment_status_panel").Parse(paymentStatusPanelTemplate))
	if err := tmpl.Execute(ctx.Writer, pageData); err != nil {
		log.Printf("admin payment panel render failed: %v", err)
		ctx.String(500, "failed to render payment status panel")
	}
}

func handlePaymentStatusUpdate(ctx *gin.Context, appDB *sql.DB, paymentService *boundlesspayment.PaymentService) {
	applyAdminSecurityHeaders(ctx)

	adminUserID, ok := resolveGoAdminAppUserID(ctx, appDB)
	if !ok {
		ctx.Redirect(302, "/"+urlPrefix+"/login")
		return
	}

	if !validateAdminCSRF(ctx) {
		ctx.String(403, "forbidden")
		return
	}

	paymentID, status, adminNotePtr, err := validatePaymentStatusUpdateInput(
		ctx.PostForm("payment_id"),
		ctx.PostForm("status"),
		ctx.PostForm("admin_note"),
	)
	if err != nil {
		ctx.String(400, "invalid input")
		return
	}

	result, err := paymentService.UpdatePaymentStatus(ctx.Request.Context(), adminUserID, paymentID, status, nil, adminNotePtr, nil)
	if err != nil {
		log.Printf("admin payment status update failed (payment_id=%s): %v", paymentID, err)
		ctx.String(400, "failed to update payment status")
		return
	}

	_ = result
	ctx.Redirect(302, "/"+urlPrefix+"/payment-panel/status")
}

func deletePendingPaymentsByIDs(dbConn *sql.DB, idArr []string) error {
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 10*time.Second)
	defer cancel()

	for _, paymentID := range idArr {
		paymentID = strings.TrimSpace(paymentID)
		if paymentID == "" {
			continue
		}

		if _, err := dbConn.ExecContext(ctx, `
			DELETE FROM payments
			WHERE payment_id = $1
			  AND status = 'pending'
		`, paymentID); err != nil {
			return fmt.Errorf("delete pending payment %s: %w", paymentID, err)
		}
	}

	return nil
}

const paymentStatusPanelTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    body{font-family:Arial,sans-serif;margin:24px;background:#f6f7fb;color:#1f2937}
    .card{background:#fff;border:1px solid #e5e7eb;border-radius:12px;padding:20px;box-shadow:0 4px 20px rgba(0,0,0,.04)}
    table{width:100%;border-collapse:collapse;margin-top:16px}
    th,td{padding:10px;border-bottom:1px solid #e5e7eb;vertical-align:top;text-align:left;font-size:14px}
    th{background:#f9fafb}
    .muted{color:#6b7280;font-size:12px}
    .row{display:flex;gap:8px;align-items:center;flex-wrap:wrap}
    input,textarea,select,button{font:inherit}
    textarea{width:100%;min-height:72px}
    .btn{padding:8px 12px;border:0;border-radius:8px;cursor:pointer}
    .btn-ok{background:#0f766e;color:#fff}
    .btn-no{background:#b91c1c;color:#fff}
    .grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}
  </style>
</head>
<body>
  <div class="card">
		<div style="margin-bottom:12px">
			<a href="{{.Prefix}}" style="display:inline-block;padding:8px 12px;border-radius:8px;background:#111827;color:#fff;text-decoration:none">Back to Admin</a>
		</div>
    <h1>{{.Title}}</h1>
    <p class="muted">Halaman ini memanggil logic service untuk update status, bukan update SQL langsung.</p>
    <table>
      <thead>
        <tr>
          <th>Payment</th>
          <th>User</th>
          <th>Package</th>
          <th>Amount</th>
          <th>Proof</th>
          <th>Action</th>
        </tr>
      </thead>
      <tbody>
        {{range .Items}}
        <tr>
          <td>
            <div><strong>{{.TransactionID}}</strong></div>
            <div class="muted">{{.PaymentID}}</div>
            <div class="muted">{{.TransactionAt}}</div>
          </td>
          <td>
            <div>{{.UserName}}</div>
            <div class="muted">{{.UserEmail}}</div>
          </td>
		  <td>
			<div>{{.PackageName}}</div>
			<div class="muted">Expired At: {{.ExpiredAt}}</div>
			{{if .IsExpired}}
			  <div class="muted" style="color:#b91c1c;font-weight:600">Status Expiry: {{.ExpiryStatus}}</div>
			{{else}}
			  <div class="muted" style="color:#0f766e;font-weight:600">Status Expiry: {{.ExpiryStatus}}</div>
			{{end}}
		  </td>
          <td>{{.Amount}}</td>
          <td>
            {{if .ProofDocumentURL}}
              <a href="{{.ProofDocumentURL}}" target="_blank" rel="noreferrer">lihat bukti</a>
            {{else}}
              <span class="muted">belum ada bukti</span>
            {{end}}
          </td>
          <td>
            <form method="post" action="{{$.Prefix}}/payment-panel/status">
							<input type="hidden" name="csrf_token" value="{{$.CSRF}}">
              <input type="hidden" name="payment_id" value="{{.PaymentID}}">
              <div class="row">
                <button class="btn btn-ok" type="submit" name="status" value="success">Approve</button>
                <button class="btn btn-no" type="submit" name="status" value="failed">Reject</button>
              </div>
              <div style="margin-top:8px">
                <textarea name="admin_note" placeholder="Admin note optional"></textarea>
              </div>
            </form>
          </td>
        </tr>
        {{end}}
      </tbody>
    </table>
  </div>
</body>
</html>`
