package service

import (
	"GoBank_PJ/bank-service/internal/models"
	"GoBank_PJ/bank-service/internal/repository/postgres"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
)

var (
	ErrCardNotFound           = errors.New("card not found")
	ErrInvalidCardData        = errors.New("invalid card data")
	ErrDecryptionFailed       = errors.New("decryption failed")
	ErrHMACVerificationFailed = errors.New("hmac verification failed")
)

type CardService struct {
	cardRepo   postgres.CardRepository
	pgpEntity  *openpgp.Entity
	hmacSecret []byte
}

func NewCardService(cardRepo postgres.CardRepository, pgpPrivateKey string, pgpPrivateKeyPassphrase string, hmacSecretKey string) (*CardService, error) {
	entity, err := readPrivateKey(pgpPrivateKey, pgpPrivateKeyPassphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to load PGP private key: %w", err)
	}
	return &CardService{
		cardRepo:   cardRepo,
		pgpEntity:  entity,
		hmacSecret: []byte(hmacSecretKey),
	}, nil
}
func (s *CardService) CreateCard(ctx context.Context, accountID uuid.UUID, isVirtual bool) (*models.Card, error) {
	cardNumber, err := generateLuhnCardNumber()
	if err != nil {
		return nil, fmt.Errorf("failed to generate card number: %w", err)
	}
	expiryDate := time.Now().AddDate(5, 0, 0).Format("01/06")
	cvvInt, err := randInt(100, 999)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CVV: %w", err)
	}
	cvv := fmt.Sprintf("%03d", cvvInt)
	encryptedCardNumber, err := s.encryptData([]byte(cardNumber))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt card number: %w", err)
	}
	encryptedExpiryDate, err := s.encryptData([]byte(expiryDate))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt expiry date: %w", err)
	}
	hashedCVV, err := bcrypt.GenerateFromPassword([]byte(cvv), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash CVV: %w", err)
	}
	hmacTag := s.generateHMAC([]byte(cardNumber))
	card := &models.Card{
		ID:                  uuid.New(),
		AccountID:           accountID,
		CardNumberEncrypted: encryptedCardNumber,
		ExpiryDateEncrypted: encryptedExpiryDate,
		CvvHash:             hashedCVV,
		HMACtag:             hmacTag,
		IsVirtual:           isVirtual,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	if err := s.cardRepo.CreateCard(ctx, card); err != nil {
		return nil, fmt.Errorf("failed to create card in repository: %w", err)
	}

	card.CardNumber = cardNumber
	card.ExpiryDate = expiryDate
	card.CVV = cvv

	return card, nil
}
func (s *CardService) GetCardByID(ctx context.Context, id uuid.UUID) (*models.Card, error) {
	card, err := s.cardRepo.GetCardByID(ctx, id)
	if err != nil {
		if err.Error() == fmt.Sprintf("card with ID %s not found", id) {
			return nil, ErrCardNotFound
		}
		return nil, fmt.Errorf("failed to get card from repository: %w", err)
	}
	decryptedCardNumberBytes, err := s.decryptData(card.CardNumberEncrypted)
	if err != nil {
		return nil, fmt.Errorf("%w: card number decryption failed: %v", ErrDecryptionFailed, err)
	}
	if !s.verifyHMAC(decryptedCardNumberBytes, card.HMACtag) {
		return nil, ErrHMACVerificationFailed
	}
	decryptedExpiryDateBytes, err := s.decryptData(card.ExpiryDateEncrypted)
	if err != nil {
		return nil, fmt.Errorf("%w: expiry date decryption failed: %v", ErrDecryptionFailed, err)
	}
	card.CardNumber = string(decryptedCardNumberBytes)
	card.ExpiryDate = string(decryptedExpiryDateBytes)
	return card, nil
}
func (s *CardService) GetCardsByAccountID(ctx context.Context, accountID uuid.UUID) ([]*models.Card, error) {
	cards, err := s.cardRepo.GetCardsByAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cards by account ID from repository: %w", err)
	}
	for _, card := range cards {
		decryptedCardNumberBytes, err := s.decryptData(card.CardNumberEncrypted)
		if err != nil {
			return nil, fmt.Errorf("%w: card number decryption failed for card %s: %v", ErrDecryptionFailed, card.ID, err)
		}
		if !s.verifyHMAC(decryptedCardNumberBytes, card.HMACtag) {
			return nil, fmt.Errorf("%w for card %s", ErrHMACVerificationFailed, card.ID)
		}
		decryptedExpiryDateBytes, err := s.decryptData(card.ExpiryDateEncrypted)
		if err != nil {
			return nil, fmt.Errorf("%w: expiry date decryption failed for card %s: %v", ErrDecryptionFailed, card.ID, err)
		}
		card.CardNumber = string(decryptedCardNumberBytes)
		card.ExpiryDate = string(decryptedExpiryDateBytes)
	}
	return cards, nil
}
func (s *CardService) DeleteCard(ctx context.Context, id uuid.UUID) error {
	if err := s.cardRepo.DeleteCard(ctx, id); err != nil {
		return fmt.Errorf("failed to delete card from repository: %w", err)
	}
	return nil
}
func (s *CardService) VerifyCVV(ctx context.Context, cardID uuid.UUID, cvv string) error {
	card, err := s.cardRepo.GetCardByID(ctx, cardID)
	if err != nil {
		return fmt.Errorf("failed to get card for CVV verification: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword(card.CvvHash, []byte(cvv)); err != nil {
		return fmt.Errorf("cvv verification failed: %w", err)
	}
	return nil
}
func readPrivateKey(privateKey string, passphrase string) (*openpgp.Entity, error) {
	keyringReader := strings.NewReader(privateKey)
	entityList, err := openpgp.ReadArmoredKeyRing(keyringReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read armored key ring: %w", err)
	}
	if len(entityList) == 0 {
		return nil, errors.New("no PGP entities found in private key")
	}
	entity := entityList[0]
	if entity.PrivateKey != nil && entity.PrivateKey.Encrypted {
		if passphrase == "" {
			return nil, errors.New("passphrase required for encrypted private key")
		}
		err := entity.PrivateKey.Decrypt([]byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt private key: %w", err)
		}
	}
	for _, subkey := range entity.Subkeys {
		if subkey.PrivateKey != nil && subkey.PrivateKey.Encrypted {
			err := subkey.PrivateKey.Decrypt([]byte(passphrase))
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt subkey: %w", err)
			}
		}
	}
	return entity, nil
}
func (s *CardService) encryptData(data []byte) (string, error) {
	if s.pgpEntity == nil {
		return "", errors.New("PGP entity not initialized")
	}
	buf := new(strings.Builder)
	armoredWriter, err := armor.Encode(buf, "PGP MESSAGE", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create armored writer: %w", err)
	}
	w, err := openpgp.Encrypt(armoredWriter, []*openpgp.Entity{s.pgpEntity}, nil, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create PGP encrypt writer: %w", err)
	}
	_, err = w.Write(data)
	if err != nil {
		return "", fmt.Errorf("failed to write data to PGP encrypt writer: %w", err)
	}
	err = w.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close PGP encrypt writer: %w", err)
	}
	err = armoredWriter.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close armored writer: %w", err)
	}
	return buf.String(), nil
}
func (s *CardService) decryptData(encryptedData string) ([]byte, error) {
	if s.pgpEntity == nil {
		return nil, errors.New("PGP entity not initialized")
	}
	block, err := armor.Decode(strings.NewReader(encryptedData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode armored PGP message: %w", err)
	}
	md, err := openpgp.ReadMessage(block.Body, openpgp.EntityList{s.pgpEntity}, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read PGP message: %w", err)
	}
	decryptedBytes, err := io.ReadAll(md.UnverifiedBody)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted data: %w", err)
	}
	return decryptedBytes, nil
}
func (s *CardService) generateHMAC(data []byte) []byte {
	h := hmac.New(sha256.New, s.hmacSecret)
	h.Write(data)
	return h.Sum(nil)
}
func (s *CardService) verifyHMAC(data, receivedHMAC []byte) bool {
	expectedHMAC := s.generateHMAC(data)
	return hmac.Equal(receivedHMAC, expectedHMAC)
}
func generateLuhnCardNumber() (string, error) {
	base := make([]int, 15)
	for i := 0; i < 15; i++ {
		digit, err := randInt(0, 9)
		if err != nil {
			return "", fmt.Errorf("failed to generate random digit for card number: %w", err)
		}
		base[i] = digit
	}
	checksum := calculateLuhnChecksum(base)
	cardNumber := ""
	for _, digit := range base {
		cardNumber += strconv.Itoa(digit)
	}
	cardNumber += strconv.Itoa(checksum)
	return cardNumber, nil
}
func calculateLuhnChecksum(digits []int) int {
	sum := 0
	double := true
	for i := len(digits) - 1; i >= 0; i-- {
		digit := digits[i]
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}
	return (10 - (sum % 10)) % 10
}
func randInt(min, max int) (int, error) {
	if min > max {
		return 0, fmt.Errorf("min cannot be greater than max")
	}
	n, err := rand.Int(rand.Reader, new(big.Int).SetInt64(int64(max-min+1)))
	if err != nil {
		return 0, fmt.Errorf("failed to generate random number: %w", err)
	}
	return int(n.Int64()) + min, nil
}
