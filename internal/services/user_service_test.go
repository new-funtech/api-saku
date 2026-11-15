package services

import (
	"context"
	"testing"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type temporaryPasswordUserRepo struct {
	users map[string]*model.User
}

func (r *temporaryPasswordUserRepo) FindAll(context.Context, int, int, map[string]interface{}) ([]model.User, int64, error) {
	return nil, 0, nil
}

func (r *temporaryPasswordUserRepo) FindByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	for _, user := range r.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *temporaryPasswordUserRepo) FindByIDWithRelations(context.Context, uuid.UUID) (*model.User, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r *temporaryPasswordUserRepo) FindByEmail(_ context.Context, email string) (*model.User, error) {
	if user := r.users[email]; user != nil {
		return user, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *temporaryPasswordUserRepo) FindByBujpID(context.Context, uuid.UUID) ([]model.User, error) {
	return nil, nil
}

func (r *temporaryPasswordUserRepo) Create(_ context.Context, user *model.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	r.users[user.Email] = user
	return nil
}

func (r *temporaryPasswordUserRepo) Update(_ context.Context, user *model.User) error {
	r.users[user.Email] = user
	return nil
}
func (r *temporaryPasswordUserRepo) Delete(context.Context, uuid.UUID) error { return nil }
func (r *temporaryPasswordUserRepo) UpdateLastLogin(context.Context, uuid.UUID) error {
	return nil
}

var _ repository.UserRepository = (*temporaryPasswordUserRepo)(nil)

func TestCreateUserGeneratesOneTimePasswordAndRequiresChange(t *testing.T) {
	repo := &temporaryPasswordUserRepo{users: make(map[string]*model.User)}
	service := NewUserService(repo, nil, nil, nil)

	created, err := service.CreateUser(context.Background(), model.CreateUserRequest{
		Email:    "guard@example.com",
		FullName: "Guard Test",
		Role:     "guard",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.TemporaryPassword == "" {
		t.Fatal("expected a temporary password in the one-time create response")
	}
	if len(created.TemporaryPassword) > 72 {
		t.Fatalf("temporary password is too long for bcrypt: %d bytes", len(created.TemporaryPassword))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repo.users["guard@example.com"].Password), []byte(created.TemporaryPassword)); err != nil {
		t.Fatalf("stored hash does not match returned temporary password: %v", err)
	}
	if !repo.users["guard@example.com"].MustChangePassword || !created.MustChangePassword {
		t.Fatal("new accounts must be forced to change their temporary password")
	}

	second, err := service.CreateUser(context.Background(), model.CreateUserRequest{
		Email:    "other@example.com",
		FullName: "Other Guard",
		Role:     "guard",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.TemporaryPassword == second.TemporaryPassword {
		t.Fatal("temporary passwords must be unique")
	}

	if err := service.ChangePassword(context.Background(), repo.users["guard@example.com"].ID, model.ChangePasswordRequest{
		CurrentPassword: created.TemporaryPassword,
		NewPassword:     "A-strong-new-password-987",
		ConfirmPassword: "A-strong-new-password-987",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.users["guard@example.com"].MustChangePassword {
		t.Fatal("changing the temporary password must clear the required-change flag")
	}
	if err := bcrypt.CompareHashAndPassword(
		[]byte(repo.users["guard@example.com"].Password),
		[]byte("A-strong-new-password-987"),
	); err != nil {
		t.Fatalf("new password was not stored: %v", err)
	}
}
