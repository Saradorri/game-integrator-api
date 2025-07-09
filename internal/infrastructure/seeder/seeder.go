package seeder

import (
	"crypto/sha256"
	"encoding/hex"
	"log"

	"github.com/saradorri/gameintegrator/internal/domain"
)

// Seeder handles database seeding operations
type Seeder struct {
	userRepo domain.UserRepository
}

// NewSeeder creates a new seeder instance
func NewSeeder(userRepo domain.UserRepository) *Seeder {
	return &Seeder{
		userRepo: userRepo,
	}
}

// SeedUsers seeds the database with initial users
func (s *Seeder) SeedUsers() error {
	log.Printf("Seeding users...")

	hash := sha256.Sum256([]byte("password123"))
	passwordHash := hex.EncodeToString(hash[:])

	users := []struct {
		id       int64
		username string
		password string
		currency string
	}{
		{34633089486, "user1", passwordHash, "USD"},
		{34679664254, "user2", passwordHash, "EUR"},
		{34616761765, "user3", passwordHash, "KES"},
		{34673635133, "user4", passwordHash, "USD"},
	}

	log.Printf("Attempting to seed users...")

	for _, u := range users {
		log.Printf("Processing user...")

		existingUser, err := s.userRepo.GetByID(u.id)
		if err != nil {
			log.Printf("Error checking existing user, skipping.")
			continue
		}

		if existingUser != nil {
			log.Printf("User already exists, skipping.")
			continue
		}

		user := &domain.User{
			ID:       u.id,
			Username: u.username,
			Password: u.password,
			Currency: u.currency,
		}

		if err := s.userRepo.Create(user); err != nil {
			log.Printf("Error creating user.")
			return err
		}
		log.Printf("Successfully created user.")
	}

	log.Printf("User seeding completed successfully")
	return nil
}
