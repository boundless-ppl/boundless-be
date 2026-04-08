package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"boundless-be/dto"
)

const (
	profileRecommendationPath    = "/recommend/profile"
	transcriptRecommendationPath = "/recommend/transcript"
	cvRecommendationPath         = "/recommend/cv"
)

type RecommendationAIClient interface {
	RecommendProfile(ctx context.Context, req dto.AIProfileRecommendationRequest) (dto.GlobalMatchAIRecommendationResponse, error)
	RecommendTranscript(ctx context.Context, req dto.AITranscriptRecommendationRequest) (dto.GlobalMatchAIRecommendationResponse, error)
	RecommendCV(ctx context.Context, req dto.AICVRecommendationRequest) (dto.GlobalMatchAIRecommendationResponse, error)
}

type HTTPRecommendationAIClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPRecommendationAIClient(baseURL string) *HTTPRecommendationAIClient {
	timeout := 180 * time.Second
	if raw := strings.TrimSpace(os.Getenv("AI_SERVICE_TIMEOUT_SECONDS")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}
	return &HTTPRecommendationAIClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *HTTPRecommendationAIClient) RecommendProfile(ctx context.Context, req dto.AIProfileRecommendationRequest) (dto.GlobalMatchAIRecommendationResponse, error) {
	return c.doMultipartRecommendation(
		ctx,
		profileRecommendationPath,
		[]multipartFilePart{
			{FieldName: "transcript_file", Header: req.TranscriptFile},
			{FieldName: "cv_file", Header: req.CVFile},
		},
		req.Preferences,
		req.AllowedCandidates,
	)
}

func (c *HTTPRecommendationAIClient) RecommendTranscript(ctx context.Context, req dto.AITranscriptRecommendationRequest) (dto.GlobalMatchAIRecommendationResponse, error) {
	return c.doMultipartRecommendation(
		ctx,
		transcriptRecommendationPath,
		[]multipartFilePart{
			{FieldName: "file", Header: req.TranscriptFile},
		},
		req.Preferences,
		req.AllowedCandidates,
	)
}

func (c *HTTPRecommendationAIClient) RecommendCV(ctx context.Context, req dto.AICVRecommendationRequest) (dto.GlobalMatchAIRecommendationResponse, error) {
	return c.doMultipartRecommendation(
		ctx,
		cvRecommendationPath,
		[]multipartFilePart{
			{FieldName: "file", Header: req.CVFile},
		},
		req.Preferences,
		req.AllowedCandidates,
	)
}

type multipartFilePart struct {
	FieldName string
	Header    *multipart.FileHeader
}

func (c *HTTPRecommendationAIClient) doMultipartRecommendation(
	ctx context.Context,
	path string,
	files []multipartFilePart,
	preferences dto.RecommendationPreferenceInput,
	allowedCandidates []dto.RecommendationAllowedCandidateInput,
) (dto.GlobalMatchAIRecommendationResponse, error) {
	requestBody, contentType, err := buildRecommendationMultipartBody(files, preferences, allowedCandidates)
	if err != nil {
		return dto.GlobalMatchAIRecommendationResponse{}, err
	}
	log.Printf("recommendation_ai_client_request path=%s base_url=%s body_bytes=%d files=%d allowed_candidates=%d", path, c.baseURL, len(requestBody), len(files), len(allowedCandidates))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(requestBody))
	if err != nil {
		return dto.GlobalMatchAIRecommendationResponse{}, fmt.Errorf("build AI multipart request: %w", err)
	}
	httpReq.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		log.Printf("recommendation_ai_client_transport_error path=%s err=%v", path, err)
		return dto.GlobalMatchAIRecommendationResponse{}, fmt.Errorf("call AI recommendation service: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("recommendation_ai_client_response path=%s status=%d", path, resp.StatusCode)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		log.Printf("recommendation_ai_client_error_body path=%s status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(body)))
		return dto.GlobalMatchAIRecommendationResponse{}, fmt.Errorf("AI recommendation service status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload dto.GlobalMatchAIRecommendationResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		log.Printf("recommendation_ai_client_decode_error path=%s err=%v", path, err)
		return dto.GlobalMatchAIRecommendationResponse{}, fmt.Errorf("decode AI recommendation response: %w", err)
	}
	log.Printf("recommendation_ai_client_success path=%s top_recommendations=%d", path, len(payload.TopRecommendations))

	return payload, nil
}

func buildRecommendationMultipartBody(
	files []multipartFilePart,
	preferences dto.RecommendationPreferenceInput,
	allowedCandidates []dto.RecommendationAllowedCandidateInput,
) ([]byte, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for _, file := range files {
		if file.Header == nil {
			continue
		}

		src, err := file.Header.Open()
		if err != nil {
			return nil, "", fmt.Errorf("open multipart file %s: %w", file.FieldName, err)
		}

		part, err := writer.CreateFormFile(file.FieldName, file.Header.Filename)
		if err != nil {
			src.Close()
			return nil, "", fmt.Errorf("create multipart form file %s: %w", file.FieldName, err)
		}

		if _, err := io.Copy(part, src); err != nil {
			src.Close()
			return nil, "", fmt.Errorf("copy multipart file %s: %w", file.FieldName, err)
		}
		src.Close()
	}

	writeMultiValueField(writer, "continents", preferences.Continents)
	writeMultiValueField(writer, "countries", preferences.Countries)
	writeMultiValueField(writer, "fields_of_study", preferences.FieldsOfStudy)
	writeSingleValueField(writer, "degree_level", preferences.DegreeLevel)
	writeMultiValueField(writer, "languages", preferences.Languages)
	writeMultiValueField(writer, "budget_preferences", preferences.BudgetPreferences)
	writeMultiValueField(writer, "scholarship_types", preferences.ScholarshipTypes)
	writeMultiValueField(writer, "start_periods", preferences.StartPeriods)
	writeSingleValueField(writer, "additional_preference", preferences.AdditionalPreference)
	if len(allowedCandidates) > 0 {
		payload, err := json.Marshal(allowedCandidates)
		if err != nil {
			return nil, "", fmt.Errorf("marshal allowed candidates: %w", err)
		}
		_ = writer.WriteField("allowed_candidates_json", string(payload))
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart writer: %w", err)
	}

	return body.Bytes(), writer.FormDataContentType(), nil
}

func writeMultiValueField(writer *multipart.Writer, field string, values []string) {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		_ = writer.WriteField(field, trimmed)
	}
}

func writeSingleValueField(writer *multipart.Writer, field, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	_ = writer.WriteField(field, trimmed)
}
