package controller

import (
	"context"
	"errors"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/middleware"
	"boundless-be/model"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PaymentService interface {
	ListPackages(ctx context.Context) ([]model.Subscription, error)
	CreatePayment(ctx context.Context, userID, subscriptionID string) (service.CreatePaymentOutput, error)
	GetMyPayment(ctx context.Context, userID, paymentID string) (service.PaymentDetailOutput, error)
	ListAdminPayments(ctx context.Context, query, status string, page, pageSize int) (service.AdminListPaymentsOutput, error)
	UpdatePaymentStatus(ctx context.Context, adminUserID string, paymentID string, status string, startDate *string, adminNote *string, proofDocumentID *string) (service.AdminUpdatePaymentStatusOutput, error)
	UploadProofForPayment(ctx context.Context, userID, paymentID string, file *multipart.FileHeader) (model.Document, error)
}

type PaymentController struct {
	paymentService PaymentService
}

func NewPaymentController(paymentService PaymentService) *PaymentController {
	return &PaymentController{paymentService: paymentService}
}

func (c *PaymentController) ListPackages(ctx *gin.Context) {
	packages, err := c.paymentService.ListPackages(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}

	items := make([]dto.SubscriptionPackageResponse, 0, len(packages))
	for _, pkg := range packages {
		items = append(items, dto.SubscriptionPackageResponse{
			SubscriptionID: pkg.SubscriptionID,
			PackageKey:     pkg.PackageKey,
			Name:           pkg.Name,
			Description:    pkg.Description,
			DurationMonths: pkg.DurationMonths,
			PriceAmount:    pkg.DiscountPriceAmount,
			NormalAmount:   pkg.NormalPriceAmount,
			Benefits:       pkg.Benefits,
		})
	}

	ctx.JSON(http.StatusOK, dto.ListSubscriptionPackagesResponse{Packages: items})
}

func (c *PaymentController) CreatePayment(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	var req dto.CreatePaymentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		return
	}

	if _, err := uuid.Parse(req.SubscriptionID); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		return
	}

	result, err := c.paymentService.CreatePayment(ctx.Request.Context(), userID.(string), req.SubscriptionID)
	if err != nil {
		log.Printf("CreatePayment error: %v", err)
		switch {
		case errors.Is(err, errs.ErrInvalidInput):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		case errors.Is(err, errs.ErrUnauthorized):
			ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		case errors.Is(err, errs.ErrSubscriptionNotFound):
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "subscription not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{
				Error: err.Error(),
			})
		}
		return
	}

	// Get normal amount from subscription data (stored in DB)
	normalAmount := int64(0)
	if result.Payment.NormalPriceSnapshot != nil {
		normalAmount = *result.Payment.NormalPriceSnapshot
	}
	expiredAtStr := ""
	if result.Payment.ExpiredAt != nil {
		expiredAtStr = result.Payment.ExpiredAt.Format(time.RFC3339)
	}
	ctx.JSON(http.StatusCreated, dto.CreatePaymentResponse{
		PaymentID:      result.Payment.PaymentID,
		TransactionID:  result.Payment.TransactionID,
		Status:         string(result.Payment.Status),
		PackageName:    result.Payment.PackageNameSnapshot,
		DurationMonths: result.Payment.DurationMonthsSnapshot,
		TotalAmount:    result.Payment.PriceAmountSnapshot,
		NormalAmount:   normalAmount,
		Benefits:       result.Payment.BenefitsSnapshot,
		QrisImageURL:   result.Payment.QrisImageURL,
		CreatedAt:      result.Payment.CreatedAt.Format(time.RFC3339),
		ExpiredAt:      expiredAtStr,
	})
}

func (c *PaymentController) GetMyPayment(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	paymentID := ctx.Param("id")
	if _, err := uuid.Parse(paymentID); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id format"})
		return
	}

	result, err := c.paymentService.GetMyPayment(ctx.Request.Context(), userID.(string), paymentID)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrInvalidInput):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		case errors.Is(err, errs.ErrPaymentNotFound):
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "payment not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	normalAmount := int64(0)
	if result.Payment.NormalPriceSnapshot != nil {
		normalAmount = *result.Payment.NormalPriceSnapshot
	}
	expiredAtStr := ""
	if result.Payment.ExpiredAt != nil {
		expiredAtStr = result.Payment.ExpiredAt.UTC().Format(time.RFC3339)
	}
	response := dto.PaymentDetailResponse{
		PaymentID:       result.Payment.PaymentID,
		TransactionID:   result.Payment.TransactionID,
		Status:          string(result.Payment.Status),
		PackageName:     result.Payment.PackageNameSnapshot,
		DurationMonths:  result.Payment.DurationMonthsSnapshot,
		TotalAmount:     result.Payment.PriceAmountSnapshot,
		NormalAmount:    normalAmount,
		Benefits:        result.Payment.BenefitsSnapshot,
		QrisImageURL:    result.Payment.QrisImageURL,
		ProofDocumentID: result.Payment.ProofDocumentID,
		CreatedAt:       result.Payment.CreatedAt.UTC().Format(time.RFC3339),
		ExpiredAt:       expiredAtStr,
	}
	if result.Payment.PaidAt != nil {
		paidAt := result.Payment.PaidAt.UTC().Format(time.RFC3339)
		response.PaidAt = &paidAt
	}
	if result.PremiumActiveAt != nil {
		active := result.PremiumActiveAt.UTC().Format(time.RFC3339)
		response.PremiumActiveAt = &active
	}
	if result.PremiumExpiredAt != nil {
		expires := result.PremiumExpiredAt.UTC().Format(time.RFC3339)
		response.PremiumExpiredAt = &expires
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *PaymentController) ListAdminPayments(ctx *gin.Context) {
	page := parseIntWithDefault(ctx.Query("page"), 1)
	pageSize := parseIntWithDefault(ctx.Query("page_size"), 20)

	result, err := c.paymentService.ListAdminPayments(
		ctx.Request.Context(),
		ctx.Query("q"),
		ctx.Query("status"),
		page,
		pageSize,
	)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrInvalidPaymentStatus):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid payment status"})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	items := make([]dto.AdminPaymentListItemResponse, 0, len(result.Items))
	for _, row := range result.Items {
		items = append(items, dto.AdminPaymentListItemResponse{
			PaymentID:        row.PaymentID,
			TransactionID:    row.TransactionID,
			UserID:           row.UserID,
			UserName:         row.UserName,
			PackageName:      row.PackageName,
			Amount:           row.Amount,
			NormalAmount:     row.NormalAmount,
			Status:           string(row.Status),
			TransactionDate:  row.TransactionAt.UTC().Format(time.RFC3339),
			ProofDocumentID:  row.ProofDocumentID,
			ProofDocumentURL: row.ProofDocumentURL,
		})
	}

	ctx.JSON(http.StatusOK, dto.AdminPaymentListResponse{Items: items})
}

func (c *PaymentController) UpdatePaymentStatus(ctx *gin.Context) {
	adminUserID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	paymentID := ctx.Param("id")
	if _, err := uuid.Parse(paymentID); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id format"})
		return
	}

	var req dto.AdminUpdatePaymentStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		return
	}

	result, err := c.paymentService.UpdatePaymentStatus(
		ctx.Request.Context(),
		adminUserID.(string),
		paymentID,
		req.Status,
		req.StartDate,
		req.AdminNote,
		req.ProofDocumentID,
	)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrInvalidInput):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		case errors.Is(err, errs.ErrInvalidPaymentStatus):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid payment status"})
		case errors.Is(err, errs.ErrPaymentNotFound):
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "payment not found"})
		case errors.Is(err, errs.ErrPaymentNotPending):
			ctx.JSON(http.StatusConflict, dto.ErrorResponse{Error: "payment already finalized"})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	response := dto.AdminUpdatePaymentStatusResponse{
		PaymentID:     result.Payment.PaymentID,
		TransactionID: result.Payment.TransactionID,
		Status:        string(result.Payment.Status),
	}
	if result.Payment.PaidAt != nil {
		paidAt := result.Payment.PaidAt.UTC().Format(time.RFC3339)
		response.PaidAt = &paidAt
	}
	if result.PremiumActiveAt != nil {
		active := result.PremiumActiveAt.UTC().Format(time.RFC3339)
		response.PremiumActiveAt = &active
	}
	if result.PremiumExpiredAt != nil {
		expires := result.PremiumExpiredAt.UTC().Format(time.RFC3339)
		response.PremiumExpiredAt = &expires
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *PaymentController) UploadPaymentProof(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	paymentID := ctx.Param("id")
	if _, err := uuid.Parse(paymentID); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id format"})
		return
	}

	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		return
	}

	doc, err := c.paymentService.UploadProofForPayment(ctx.Request.Context(), userID.(string), paymentID, file)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrInvalidInput):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		case errors.Is(err, errs.ErrPaymentNotFound):
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "payment not found"})
		case errors.Is(err, errs.ErrPaymentNotPending):
			ctx.JSON(http.StatusConflict, dto.ErrorResponse{Error: "payment already finalized"})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	ctx.JSON(http.StatusCreated, dto.UploadPaymentProofResponse{
		DocumentID:       doc.DocumentID,
		OriginalFilename: doc.OriginalFilename,
		PublicURL:        doc.PublicURL,
		MIMEType:         doc.MIMEType,
		SizeBytes:        doc.SizeBytes,
		DocumentType:     string(doc.DocumentType),
		UploadedAt:       doc.UploadedAt.UTC().Format(time.RFC3339),
	})
}

func parseIntWithDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
