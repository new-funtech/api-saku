package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	config.LoadEnv()
	config.ConnectDatabase()
	config.RunMigrations()

	email := config.GetEnv("BPRKS_EMAIL", "bprks@hrmis.local")
	password := config.GetEnv("BPRKS_PASSWORD", "Bprks#12345")
	fullName := config.GetEnv("BPRKS_FULL_NAME", "BPRKS Reviewer")

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}

	var existing model.User
	err = config.DB.Where("email = ?", email).First(&existing).Error
	switch {
	case err == nil:
		existing.Role = constants.RoleBprks
		existing.Status = "active"
		existing.FullName = fullName
		existing.Password = string(hashed)
		if err := config.DB.Save(&existing).Error; err != nil {
			log.Fatalf("failed to update existing BPRKS user: %v", err)
		}
		fmt.Printf("BPRKS user already existed — updated it instead of creating a duplicate.\n")
	case errors.Is(err, gorm.ErrRecordNotFound):
		user := model.User{
			Email:    email,
			Password: string(hashed),
			Role:     constants.RoleBprks,
			FullName: fullName,
			Status:   "active",
		}
		if err := config.DB.Create(&user).Error; err != nil {
			log.Fatalf("failed to create BPRKS user: %v", err)
		}
		fmt.Println("BPRKS user created.")
	default:
		log.Fatalf("failed to look up existing user: %v", err)
	}

	fmt.Println()
	fmt.Println("  Email:    ", email)
	fmt.Println("  Password: ", password)
	fmt.Println("  Role:     ", constants.RoleBprks)
	fmt.Println()
	fmt.Println("Please change the password after first login.")
}
