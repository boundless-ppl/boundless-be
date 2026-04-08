package service

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"boundless-be/repository"
)

type PaymentEmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

type SMTPEmailSender struct {
	host      string
	port      int
	username  string
	password  string
	fromName  string
	fromEmail string
}

func NewSMTPEmailSenderFromEnv() (*SMTPEmailSender, error) {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if host == "" {
		return nil, fmt.Errorf("SMTP_HOST is required")
	}

	port := 587
	if raw := strings.TrimSpace(os.Getenv("SMTP_PORT")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("invalid SMTP_PORT")
		}
		port = parsed
	}

	username := strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	password := strings.TrimSpace(os.Getenv("SMTP_PASSWORD"))
	fromEmail := strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL"))
	if fromEmail == "" {
		fromEmail = username
	}
	if fromEmail == "" {
		return nil, fmt.Errorf("SMTP_FROM_EMAIL is required")
	}

	fromName := strings.TrimSpace(os.Getenv("SMTP_FROM_NAME"))
	if fromName == "" {
		fromName = "Boundless"
	}

	return &SMTPEmailSender{
		host:      host,
		port:      port,
		username:  username,
		password:  password,
		fromName:  fromName,
		fromEmail: fromEmail,
	}, nil
}

func (s *SMTPEmailSender) Send(ctx context.Context, to, subject, body string) error {
	_ = ctx
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("missing recipient")
	}

	headers := map[string]string{
		"From":         fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		"To":           to,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=UTF-8",
	}

	var message bytes.Buffer
	for key, value := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}
	message.WriteString("\r\n")
	message.WriteString(body)

	address := fmt.Sprintf("%s:%d", s.host, s.port)
	var auth smtp.Auth
	if s.username != "" && s.password != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	return smtp.SendMail(address, auth, s.fromEmail, []string{to}, message.Bytes())
}

type PaymentNotificationService struct {
	repo       repository.PaymentRepository
	sender     PaymentEmailSender
	adminEmail string
	limit      int
}

func NewPaymentNotificationService(repo repository.PaymentRepository, sender PaymentEmailSender, adminEmail string) *PaymentNotificationService {
	return &PaymentNotificationService{
		repo:       repo,
		sender:     sender,
		adminEmail: strings.TrimSpace(adminEmail),
		limit:      50,
	}
}

func (s *PaymentNotificationService) RunOnce(ctx context.Context) error {
	if s == nil || s.repo == nil || s.sender == nil || s.adminEmail == "" {
		log.Printf("payment notifier skipped: missing dependency or admin email")
		return nil
	}

	notifications, err := s.repo.ListPendingPaymentNotifications(ctx, s.limit)
	if err != nil {
		log.Printf("payment notifier fetch failed: %v", err)
		return err
	}
	if len(notifications) == 0 {
		log.Printf("payment notifier: no pending payments to notify")
		return nil
	}

	log.Printf("payment notifier: processing %d pending payments", len(notifications))

	var lastErr error
	for _, item := range notifications {
		subject, body := buildPaymentNotificationEmail(item)
		log.Printf("payment notifier: sending email for payment_id=%s to=%s", item.PaymentID, s.adminEmail)
		if err := s.sender.Send(ctx, s.adminEmail, subject, body); err != nil {
			log.Printf("payment notifier: send failed payment_id=%s err=%v", item.PaymentID, err)
			lastErr = err
			continue
		}
		log.Printf("payment notifier: send accepted payment_id=%s", item.PaymentID)

		if err := s.repo.MarkPaymentNotificationSent(ctx, item.PaymentID, time.Now().UTC()); err != nil {
			log.Printf("payment notifier: mark notified failed payment_id=%s err=%v", item.PaymentID, err)
			lastErr = err
			continue
		}
		log.Printf("payment notifier: marked notified payment_id=%s", item.PaymentID)
	}

	return lastErr
}

func buildPaymentNotificationEmail(item repository.PendingPaymentNotification) (string, string) {
	subject := fmt.Sprintf("[Payment Pending] %s - %s", item.UserName, item.TransactionID)
	body := fmt.Sprintf(
		"Ada payment subscription yang menunggu review.\n\nUser: %s (%s)\nUser ID: %s\nTransaction ID: %s\nPayment ID: %s\nPackage: %s\nAmount: %d\nCreated At: %s\nProof URL: %s\n\nSilakan cek bukti transfer lalu update status payment di admin panel.",
		item.UserName,
		item.UserEmail,
		item.UserID,
		item.TransactionID,
		item.PaymentID,
		item.PackageName,
		item.Amount,
		item.CreatedAt.UTC().Format(time.RFC3339),
		item.ProofDocumentURL,
	)
	return subject, body
}
