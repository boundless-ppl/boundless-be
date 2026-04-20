package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"boundless-be/dto"
	"boundless-be/service"
)

func TestHTTPRecommendationAIClientRoutesEachModeToExpectedEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		call         func(context.Context, *service.HTTPRecommendationAIClient) (dto.GlobalMatchAIRecommendationResponse, error)
		expectFile   string
		expectFields map[string][]string
	}{
		{
			name: "profile",
			path: "/recommend/profile",
			call: func(ctx context.Context, client *service.HTTPRecommendationAIClient) (dto.GlobalMatchAIRecommendationResponse, error) {
				return client.RecommendProfile(ctx, dto.AIProfileRecommendationRequest{
					TranscriptFile: makeFileHeader(t, "transcript_file", "transcript.pdf", []byte("transcript")),
					CVFile:         makeFileHeader(t, "cv_file", "cv.pdf", []byte("cv")),
					Preferences: dto.RecommendationPreferenceInput{
						Countries:   []string{"Japan"},
						DegreeLevel: "Bachelor",
					},
				})
			},
			expectFile: "transcript_file",
			expectFields: map[string][]string{
				"countries":    {"Japan"},
				"degree_level": {"Bachelor"},
			},
		},
		{
			name: "transcript",
			path: "/recommend/transcript",
			call: func(ctx context.Context, client *service.HTTPRecommendationAIClient) (dto.GlobalMatchAIRecommendationResponse, error) {
				return client.RecommendTranscript(ctx, dto.AITranscriptRecommendationRequest{
					TranscriptFile: makeFileHeader(t, "transcript_file", "transcript.pdf", []byte("transcript")),
					Preferences: dto.RecommendationPreferenceInput{
						FieldsOfStudy: []string{"Data Science"},
					},
				})
			},
			expectFile: "file",
			expectFields: map[string][]string{
				"fields_of_study": {"Data Science"},
			},
		},
		{
			name: "cv",
			path: "/recommend/cv",
			call: func(ctx context.Context, client *service.HTTPRecommendationAIClient) (dto.GlobalMatchAIRecommendationResponse, error) {
				return client.RecommendCV(ctx, dto.AICVRecommendationRequest{
					CVFile: makeFileHeader(t, "cv_file", "cv.pdf", []byte("cv")),
					Preferences: dto.RecommendationPreferenceInput{
						Languages: []string{"English"},
					},
				})
			},
			expectFile: "file",
			expectFields: map[string][]string{
				"languages": {"English"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.path {
					t.Fatalf("expected path %s, got %s", tt.path, r.URL.Path)
				}

				mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
				if err != nil {
					t.Fatalf("parse media type: %v", err)
				}
				if mediaType != "multipart/form-data" {
					t.Fatalf("expected multipart/form-data, got %s", mediaType)
				}

				reader := multipart.NewReader(r.Body, params["boundary"])
				form, err := reader.ReadForm(10 << 20)
				if err != nil {
					t.Fatalf("read form: %v", err)
				}

				files := form.File[tt.expectFile]
				if len(files) != 1 {
					t.Fatalf("expected one file for %s, got %d", tt.expectFile, len(files))
				}

				for key, expected := range tt.expectFields {
					assertFormValues(t, form.Value[key], expected)
				}

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(dto.GlobalMatchAIRecommendationResponse{
					StudentProfileSummary: dto.GlobalMatchAIStudentProfileSummaryResponse{
						RawText: "summary",
					},
					TopRecommendations: []dto.GlobalMatchAITopRecommendationResponse{
						{
							Rank:           1,
							UniversityName: "Test University",
							ProgramName:    "AI",
							Country:        "Japan",
						},
					},
					SelectionReasoning: "reason",
					ApplicationStrategy: dto.GlobalMatchAIApplicationStrategyResponse{
						Target: "strategy",
					},
					FinalNotes: []string{"notes"},
				})
			}))
			defer server.Close()

			client := service.NewHTTPRecommendationAIClient(server.URL)
			resp, err := tt.call(context.Background(), client)
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if len(resp.TopRecommendations) != 1 || resp.TopRecommendations[0].UniversityName != "Test University" {
				t.Fatalf("unexpected AI response: %#v", resp.TopRecommendations)
			}
		})
	}
}

func TestHTTPRecommendationAIClientForwardsAllMultipartPreferenceFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse media type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("expected multipart/form-data, got %s", mediaType)
		}

		reader := multipart.NewReader(r.Body, params["boundary"])
		form, err := reader.ReadForm(10 << 20)
		if err != nil {
			t.Fatalf("read form: %v", err)
		}

		assertFormValues(t, form.Value["continents"], []string{"Asia", "Europe"})
		assertFormValues(t, form.Value["countries"], []string{"Japan", "Singapore"})
		assertFormValues(t, form.Value["fields_of_study"], []string{"Computer Science"})
		assertFormValues(t, form.Value["languages"], []string{"English"})
		assertFormValues(t, form.Value["budget_preferences"], []string{"Affordable"})
		assertFormValues(t, form.Value["scholarship_types"], []string{"Merit-based"})
		assertFormValues(t, form.Value["start_periods"], []string{"Fall 2026"})

		if got := firstFormValue(form.Value["degree_level"]); got != "Master" {
			t.Fatalf("expected degree_level Master, got %q", got)
		}
		if got := firstFormValue(form.Value["additional_preference"]); got != "Need strong AI labs" {
			t.Fatalf("expected additional_preference, got %q", got)
		}
		var allowedCandidates []dto.AIAllowedCandidate
		if err := json.Unmarshal([]byte(firstFormValue(form.Value["allowed_candidates_json"])), &allowedCandidates); err != nil {
			t.Fatalf("unmarshal allowed candidates: %v", err)
		}
		if len(allowedCandidates) != 1 || allowedCandidates[0].ProgramID != "program-1" {
			t.Fatalf("expected allowed candidates to be forwarded, got %#v", allowedCandidates)
		}

		if len(form.File["transcript_file"]) != 1 {
			t.Fatalf("expected transcript_file upload, got %#v", form.File["transcript_file"])
		}
		if len(form.File["cv_file"]) != 1 {
			t.Fatalf("expected cv_file upload, got %#v", form.File["cv_file"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(dto.GlobalMatchAIRecommendationResponse{})
	}))
	defer server.Close()

	client := service.NewHTTPRecommendationAIClient(server.URL)
	_, err := client.RecommendProfile(context.Background(), dto.AIProfileRecommendationRequest{
		TranscriptFile: makeFileHeader(t, "transcript_file", "transcript.pdf", []byte("transcript-content")),
		CVFile:         makeFileHeader(t, "cv_file", "cv.pdf", []byte("cv-content")),
		Preferences: dto.RecommendationPreferenceInput{
			Continents:           []string{"Asia", " ", "Europe"},
			Countries:            []string{"Japan", "Singapore"},
			FieldsOfStudy:        []string{"Computer Science"},
			DegreeLevel:          "Master",
			Languages:            []string{"English"},
			BudgetPreferences:    []string{"Affordable"},
			ScholarshipTypes:     []string{"Merit-based"},
			StartPeriods:         []string{"Fall 2026"},
			AdditionalPreference: "Need strong AI labs",
		},
		AllowedCandidates: []dto.AIAllowedCandidate{{
			ProgramID:             "program-1",
			ProgramName:           "Computer Science",
			UniversityName:        "University A",
			Country:               "Japan",
			OfficialProgramURL:    "https://unia.example/cs",
			OfficialUniversityURL: "https://unia.example",
		}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestHTTPRecommendationAIClientReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream failed", http.StatusBadGateway)
	}))
	defer server.Close()

	client := service.NewHTTPRecommendationAIClient(server.URL)
	_, err := client.RecommendTranscript(context.Background(), dto.AITranscriptRecommendationRequest{
		TranscriptFile: makeFileHeader(t, "transcript_file", "transcript.pdf", []byte("transcript")),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got == "" || !bytes.Contains([]byte(got), []byte("502")) {
		t.Fatalf("expected upstream status in error, got %q", got)
	}
}

func assertFormValues(t *testing.T, got, expected []string) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf("expected %d values, got %d: %#v", len(expected), len(got), got)
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("expected value %q at index %d, got %q", expected[i], i, got[i])
		}
	}
}

func firstFormValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
