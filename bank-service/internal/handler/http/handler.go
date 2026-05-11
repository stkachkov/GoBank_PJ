package http

import (
	"GoBank_PJ/bank-service/internal/service"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
type Handler struct {
	AuthService         *service.AuthService
	AccountService      *service.AccountService
	CardService         *service.CardService
	CreditService       *service.CreditService
	AnalyticsService    *service.AnalyticsService
	NotificationService *service.NotificationService
	CentralBankService  *service.CentralBankService
}

func NewHandler(
	authSvc *service.AuthService,
	accountSvc *service.AccountService,
	cardSvc *service.CardService,
	creditSvc *service.CreditService,
	analyticsSvc *service.AnalyticsService,
	notificationSvc *service.NotificationService,
	centralBankSvc *service.CentralBankService,
) *Handler {
	return &Handler{
		AuthService:         authSvc,
		AccountService:      accountSvc,
		CardService:         cardSvc,
		CreditService:       creditSvc,
		AnalyticsService:    analyticsSvc,
		NotificationService: notificationSvc,
		CentralBankService:  centralBankSvc,
	}
}
func sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
	}
}
func sendErrorResponse(w http.ResponseWriter, statusCode int, message string, err error) {
	resp := Response{
		Status:  "error",
		Message: message,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	sendJSONResponse(w, statusCode, resp)
}
func sendSuccessResponse(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	resp := Response{
		Status:  "success",
		Message: message,
		Data:    data,
	}
	sendJSONResponse(w, statusCode, resp)
}

type contextKey string

const userIDContextKey contextKey = "userID"

func getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	userID, ok := ctx.Value(userIDContextKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("user ID not found in context")
	}
	return userID, nil
}

type CreateAccountRequest struct {
	Currency string `json:"currency"`
}

func (h *Handler) CreateAccountHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if req.Currency == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Currency is required", nil)
		return
	}
	if req.Currency != "RUB" {
		sendErrorResponse(w, http.StatusBadRequest, "Only RUB currency is supported", nil)
		return
	}
	account, err := h.AccountService.CreateAccount(r.Context(), userID, req.Currency)
	if err != nil {
		if errors.Is(err, service.ErrAccountAlreadyExists) {
			sendErrorResponse(w, http.StatusConflict, err.Error(), err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to create account", err)
		return
	}
	sendSuccessResponse(w, http.StatusCreated, "Account created successfully", account)
}
func (h *Handler) GetUserAccountsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	accounts, err := h.AccountService.GetAccountsByUserID(r.Context(), userID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve accounts", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "Accounts retrieved successfully", accounts)
}

type DepositRequest struct {
	Amount float64 `json:"amount"`
}

func (h *Handler) DepositHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	vars := mux.Vars(r)
	accountIDStr := vars["id"]
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid account ID format", err)
		return
	}
	var req DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if req.Amount <= 0 {
		sendErrorResponse(w, http.StatusBadRequest, "Deposit amount must be positive", nil)
		return
	}
	account, err := h.AccountService.GetAccountByID(r.Context(), accountID)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "Account not found", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve account for deposit", err)
		return
	}
	if account.UserID.String() != userID.String() {
		sendErrorResponse(w, http.StatusForbidden, "Access denied to this account", nil)
		return
	}
	updatedAccount, err := h.AccountService.Deposit(r.Context(), accountID, req.Amount)
	if err != nil {
		if errors.Is(err, service.ErrInvalidAmount) {
			sendErrorResponse(w, http.StatusBadRequest, err.Error(), err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to deposit funds", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "Funds deposited successfully", updatedAccount)
}

type WithdrawRequest struct {
	Amount float64 `json:"amount"`
}

func (h *Handler) WithdrawHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	vars := mux.Vars(r)
	accountIDStr := vars["id"]
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid account ID format", err)
		return
	}
	var req WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if req.Amount <= 0 {
		sendErrorResponse(w, http.StatusBadRequest, "Withdrawal amount must be positive", nil)
		return
	}
	account, err := h.AccountService.GetAccountByID(r.Context(), accountID)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "Account not found", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve account for withdrawal", err)
		return
	}
	if account.UserID.String() != userID.String() {
		sendErrorResponse(w, http.StatusForbidden, "Access denied to this account", nil)
		return
	}
	updatedAccount, err := h.AccountService.Withdraw(r.Context(), accountID, req.Amount)
	if err != nil {
		if errors.Is(err, service.ErrInvalidAmount) || errors.Is(err, service.ErrInsufficientFunds) {
			sendErrorResponse(w, http.StatusBadRequest, err.Error(), err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to withdraw funds", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "Funds withdrawn successfully", updatedAccount)
}

type TransferRequest struct {
	ToAccountID string  `json:"to_account_id"`
	Amount      float64 `json:"amount"`
}

func (h *Handler) TransferHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	vars := mux.Vars(r)
	fromAccountIDStr := vars["id"]
	fromAccountID, err := uuid.Parse(fromAccountIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid source account ID format", err)
		return
	}
	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	toAccountID, err := uuid.Parse(req.ToAccountID)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid destination account ID format", err)
		return
	}
	if req.Amount <= 0 {
		sendErrorResponse(w, http.StatusBadRequest, "Transfer amount must be positive", nil)
		return
	}
	fromAccount, err := h.AccountService.GetAccountByID(r.Context(), fromAccountID)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "Source account not found", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve source account for transfer", err)
		return
	}
	if fromAccount.UserID.String() != userID.String() {
		sendErrorResponse(w, http.StatusForbidden, "Access denied to source account", nil)
		return
	}
	err = h.AccountService.Transfer(r.Context(), fromAccountID, toAccountID, req.Amount)
	if err != nil {
		if errors.Is(err, service.ErrInvalidAmount) || errors.Is(err, service.ErrInsufficientFunds) || errors.Is(err, service.ErrSelfTransfer) || errors.Is(err, service.ErrCurrencyMismatch) {
			sendErrorResponse(w, http.StatusBadRequest, err.Error(), err)
			return
		}
		if strings.Contains(err.Error(), "destination account not found") {
			sendErrorResponse(w, http.StatusNotFound, "Destination account not found", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to transfer funds", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "Funds transferred successfully", nil)
}

type CreateCardRequest struct {
	AccountID uuid.UUID `json:"account_id"`
	IsVirtual bool      `json:"is_virtual"`
}

func (h *Handler) CreateCardHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	var req CreateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	account, err := h.AccountService.GetAccountByID(r.Context(), req.AccountID)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "Account not found", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve account for card creation", err)
		return
	}
	if account.UserID.String() != userID.String() {
		sendErrorResponse(w, http.StatusForbidden, "Access denied to this account", nil)
		return
	}
	card, err := h.CardService.CreateCard(r.Context(), req.AccountID, req.IsVirtual)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to create card", err)
		return
	}
	sendSuccessResponse(w, http.StatusCreated, "Card created successfully", card)
}
func (h *Handler) GetCardsByAccountHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	vars := mux.Vars(r)
	accountIDStr := vars["id"]
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid account ID format", err)
		return
	}
	account, err := h.AccountService.GetAccountByID(r.Context(), accountID)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "Account not found", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve account for card listing", err)
		return
	}
	if account.UserID.String() != userID.String() {
		sendErrorResponse(w, http.StatusForbidden, "Access denied to this account", nil)
		return
	}
	cards, err := h.CardService.GetCardsByAccountID(r.Context(), accountID)
	if err != nil {
		if errors.Is(err, service.ErrDecryptionFailed) || errors.Is(err, service.ErrHMACVerificationFailed) {
			sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve card data due to security error", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve cards", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "Cards retrieved successfully", cards)
}

type VerifyCVVRequest struct {
	CVV string `json:"cvv"`
}

func (h *Handler) VerifyCVVHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	vars := mux.Vars(r)
	cardIDStr := vars["id"]
	cardID, err := uuid.Parse(cardIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid card ID format", err)
		return
	}
	var req VerifyCVVRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if req.CVV == "" {
		sendErrorResponse(w, http.StatusBadRequest, "CVV is required", nil)
		return
	}
	card, err := h.CardService.GetCardByID(r.Context(), cardID)
	if err != nil {
		if errors.Is(err, service.ErrCardNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "Card not found", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve card for CVV verification", err)
		return
	}
	account, err := h.AccountService.GetAccountByID(r.Context(), card.AccountID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve account for card's owner", err)
		return
	}
	if account.UserID.String() != userID.String() {
		sendErrorResponse(w, http.StatusForbidden, "Access denied to this card", nil)
		return
	}
	err = h.CardService.VerifyCVV(r.Context(), cardID, req.CVV)
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "CVV verification failed", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "CVV verified successfully", nil)
}

func (h *Handler) DeleteCardHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}

	vars := mux.Vars(r)
	cardIDStr := vars["id"]
	cardID, err := uuid.Parse(cardIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid card ID format", err)
		return
	}

	card, err := h.CardService.GetCardByID(r.Context(), cardID)
	if err != nil {
		if errors.Is(err, service.ErrCardNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "Card not found", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve card for deletion", err)
		return
	}

	account, err := h.AccountService.GetAccountByID(r.Context(), card.AccountID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve account for card's owner check", err)
		return
	}

	if account.UserID != userID {
		sendErrorResponse(w, http.StatusForbidden, "Access denied: you do not own this card", nil)
		return
	}

	if err := h.CardService.DeleteCard(r.Context(), cardID); err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to delete card", err)
		return
	}

	sendSuccessResponse(w, http.StatusOK, "Card deleted successfully", nil)
}


type ApplyForCreditRequest struct {
	AccountID    uuid.UUID `json:"account_id"`
	Amount       float64   `json:"amount"`
	TermMonths   int       `json:"term_months"`
	InterestRate float64   `json:"interest_rate"`
}

func (h *Handler) ApplyForCreditHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	var req ApplyForCreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	account, err := h.AccountService.GetAccountByID(r.Context(), req.AccountID)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "Account not found", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve account for credit application", err)
		return
	}
	if account.UserID.String() != userID.String() {
		sendErrorResponse(w, http.StatusForbidden, "Access denied to this account", nil)
		return
	}
	credit, err := h.CreditService.ApplyForCredit(r.Context(), userID, req.AccountID, req.Amount, req.TermMonths, req.InterestRate)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCreditAmount) || errors.Is(err, service.ErrInvalidTerm) {
			sendErrorResponse(w, http.StatusBadRequest, err.Error(), err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to apply for credit", err)
		return
	}
	sendSuccessResponse(w, http.StatusCreated, "Credit application successful", credit)
}
func (h *Handler) GetUserCreditsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	credits, err := h.CreditService.GetCreditsByUserID(r.Context(), userID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve credits", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "Credits retrieved successfully", credits)
}
func (h *Handler) GetCreditScheduleHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	vars := mux.Vars(r)
	creditIDStr := vars["id"]
	creditID, err := uuid.Parse(creditIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid credit ID format", err)
		return
	}
	credit, err := h.CreditService.GetCreditByID(r.Context(), creditID)
	if err != nil {
		if errors.Is(err, service.ErrCreditNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "Credit not found", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve credit for schedule", err)
		return
	}
	if credit.UserID.String() != userID.String() {
		sendErrorResponse(w, http.StatusForbidden, "Access denied to this credit", nil)
		return
	}
	schedule, err := h.CreditService.GetPaymentSchedule(r.Context(), creditID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve payment schedule", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "Payment schedule retrieved successfully", schedule)
}
func (h *Handler) GetMonthlyIncomeExpenseHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")
	year, err := strconv.Atoi(yearStr)
	if err != nil || year == 0 {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid year parameter", nil)
		return
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil || month <= 0 || month > 12 {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid month parameter (1-12)", nil)
		return
	}
	income, expense, err := h.AnalyticsService.GetMonthlyIncomeExpense(r.Context(), userID, year, month)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve income and expense data", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "Monthly income and expense retrieved successfully", map[string]float64{
		"income":  income,
		"expense": expense,
	})
}
func (h *Handler) GetCreditLoadAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	analytics, err := h.AnalyticsService.GetCreditLoadAnalytics(r.Context(), userID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve credit load analytics", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "Credit load analytics retrieved successfully", analytics)
}
func (h *Handler) PredictAccountBalanceHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required", err)
		return
	}
	vars := mux.Vars(r)
	accountIDStr := vars["id"]
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid account ID format", err)
		return
	}
	daysStr := r.URL.Query().Get("days")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid number of days for prediction", nil)
		return
	}
	account, err := h.AccountService.GetAccountByID(r.Context(), accountID)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "Account not found", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve account for prediction", err)
		return
	}
	if account.UserID.String() != userID.String() {
		sendErrorResponse(w, http.StatusForbidden, "Access denied to this account", nil)
		return
	}
	predictedBalance, err := h.AnalyticsService.PredictAccountBalance(r.Context(), accountID, days)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to predict account balance", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "Account balance prediction successful", map[string]interface{}{
		"predicted_balance": predictedBalance,
		"prediction_days":   days,
	})
}
func (h *Handler) GetCentralBankKeyRateHandler(w http.ResponseWriter, r *http.Request) {
	rate, err := h.CentralBankService.GetKeyRate(r.Context())
	if err != nil {
		if errors.Is(err, service.ErrCentralBankServiceUnavailable) || errors.Is(err, service.ErrFailedToParseXMLResponse) || errors.Is(err, service.ErrKeyRateNotFound) {
			sendErrorResponse(w, http.StatusBadGateway, err.Error(), err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve central bank key rate", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "Central Bank key rate retrieved successfully", map[string]float64{
		"key_rate": rate,
	})
}
