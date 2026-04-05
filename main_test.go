package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestMainExitsWhenDatabaseURLMissing(t *testing.T) {
	if os.Getenv("BE_MAIN_SUBPROCESS") == "1" {
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainExitsWhenDatabaseURLMissing")
	cmd.Env = append(os.Environ(),
		"BE_MAIN_SUBPROCESS=1",
		"AUTH_SECRET=test-secret",
		"DATABASE_URL=",
	)
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to fail")
	}
}
