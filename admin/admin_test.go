package admin

import "testing"

func TestLoadSuperuserCredentialsSuccess(t *testing.T) {
	env := func(key string) string {
		switch key {
		case "SUPERUSER_EMAIL":
			return "  Admin@Example.com  "
		case "SUPERUSER_PASSWORD":
			return "StrongPass123!"
		default:
			return ""
		}
	}

	email, password, err := loadSuperuserCredentials(env)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if email != "admin@example.com" {
		t.Fatalf("expected normalized email admin@example.com, got %s", email)
	}
	if password != "StrongPass123!" {
		t.Fatalf("expected unchanged password, got %s", password)
	}
}

func TestLoadSuperuserCredentialsMissingEnv(t *testing.T) {
	env := func(key string) string {
		return ""
	}

	_, _, err := loadSuperuserCredentials(env)
	if err == nil {
		t.Fatal("expected error when env vars are missing")
	}
}

func TestLoadSuperuserCredentialsRejectsAdminAdmin(t *testing.T) {
	env := func(key string) string {
		switch key {
		case "SUPERUSER_EMAIL":
			return "admin"
		case "SUPERUSER_PASSWORD":
			return "admin"
		default:
			return ""
		}
	}

	_, _, err := loadSuperuserCredentials(env)
	if err == nil {
		t.Fatal("expected error for insecure admin/admin credentials")
	}
}

func TestIsPaymentPanelTable(t *testing.T) {
	if !isPaymentPanelTable("subscriptions") {
		t.Fatal("expected subscriptions to be payment panel table")
	}
	if isPaymentPanelTable("dream_tracker") {
		t.Fatal("did not expect dream_tracker to be payment panel table")
	}
}
