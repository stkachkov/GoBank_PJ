package service

import (
	"GoBank_PJ/bank-service/internal/models"
	"GoBank_PJ/bank-service/internal/repository/postgres"
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrUserExists      = errors.New("user with this username or email already exists")
)

type AuthService struct {
	userRepo  postgres.UserRepository
	jwtSecret []byte
}

func NewAuthService(userRepo postgres.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtSecret: []byte(jwtSecret),
	}
}
func (s *AuthService) Register(ctx context.Context, req models.RegisterRequest) (*models.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}
	createdUser, err := s.userRepo.CreateUser(ctx, user)
	if err != nil {
		return nil, ErrUserExists
	}
	return createdUser, nil
}
func (s *AuthService) Login(ctx context.Context, req models.LoginRequest) (string, error) {
	user, err := s.userRepo.FindUserByUsername(ctx, req.Username)
	if err != nil {
		return "", ErrUserNotFound
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return "", ErrInvalidPassword
	}
	token, err := s.generateJWT(user.ID)
	if err != nil {
		return "", err
	}
	return token, nil
}
func (s *AuthService) generateJWT(userID string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}
