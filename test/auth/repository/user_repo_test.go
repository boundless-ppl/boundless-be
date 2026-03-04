package repository_test

import (
	"testing"

	"boundless-be/repository"
)

func TestRepositoryImplementsContractRepo(t *testing.T) {
	var _ repository.UserRepository = (*repository.DBUserRepository)(nil)
}

func TestRepositoryConstructorRepo(t *testing.T) {
	repo := repository.NewUserRepository(nil)
	if repo == nil {
		t.Fatal("expected non nil repository")
	}
}
