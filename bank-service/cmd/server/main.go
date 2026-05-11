package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"GoBank_PJ/bank-service/internal/config"
	httphandler "GoBank_PJ/bank-service/internal/handler/http"
	"GoBank_PJ/bank-service/internal/repository/postgres"
	"GoBank_PJ/bank-service/internal/service"

	"github.com/gorilla/mux"
)

func init() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
}

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		logrus.Fatalf("Error loading configuration: %v", err)
	}
	logrus.Infof("Configuration loaded. Server Port: %d", cfg.ServerPort)

	db, err := postgres.NewDB(cfg.DatabaseURL)
	if err != nil {
		logrus.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	userRepo := postgres.NewPostgresUserRepository(db.DBTX)
	accountRepo := postgres.NewPostgresAccountRepository(db.DBTX)
	cardRepo := postgres.NewPostgresCardRepository(db.DBTX)
	transactionRepo := postgres.NewPostgresTransactionRepository(db.DBTX)
	creditRepo := postgres.NewPostgresCreditRepository(db.DBTX)
	paymentScheduleRepo := postgres.NewPostgresPaymentScheduleRepository(db.DBTX)
	logrus.Info("Repositories initialized.")

	authService := service.NewAuthService(userRepo, cfg.JwtSecret)
	notificationService := service.NewNotificationService(service.SMTPSettings{
		Host:     cfg.SmtpHost,
		Port:     cfg.SmtpPort,
		Username: cfg.SmtpUsername,
		Password: cfg.SmtpPassword,
		From:     cfg.SmtpFrom,
	})
	accountService := service.NewAccountService(db, accountRepo, transactionRepo, userRepo, notificationService)

	cardService, err := service.NewCardService(cardRepo, cfg.PgpPrivateKey, cfg.PgpPrivateKeyPassphrase, cfg.HmacSecretKey)
	if err != nil {
		logrus.Fatalf("Error initializing card service: %v", err)
	}

	creditService := service.NewCreditService(db, creditRepo, paymentScheduleRepo, accountRepo, transactionRepo)
	analyticsService := service.NewAnalyticsService(accountRepo, transactionRepo, creditRepo, paymentScheduleRepo)
	centralBankService := service.NewCentralBankService()
	logrus.Info("Services initialized.")

	schedulerService := service.NewSchedulerService(creditService, accountService, userRepo, notificationService)
	ctx, cancel := context.WithCancel(context.Background())
	schedulerService.Start(ctx)
	defer cancel()
	defer schedulerService.Stop()

	handler := httphandler.NewHandler(
		authService,
		accountService,
		cardService,
		creditService,
		analyticsService,
		notificationService,
		centralBankService,
	)
	logrus.Info("HTTP Handlers initialized.")

	r := mux.NewRouter()

	r.HandleFunc("/register", handler.RegisterHandler).Methods("POST")
	r.HandleFunc("/login", handler.LoginHandler).Methods("POST")
	r.HandleFunc("/cbr/key-rate", handler.GetCentralBankKeyRateHandler).Methods("GET")

	protected := r.PathPrefix("/").Subrouter()
	protected.Use(func(next http.Handler) http.Handler {
		return httphandler.AuthMiddleware(cfg, next)
	})

	protected.HandleFunc("/accounts", handler.CreateAccountHandler).Methods("POST")
	protected.HandleFunc("/accounts", handler.GetUserAccountsHandler).Methods("GET")
	protected.HandleFunc("/accounts/{id}/deposit", handler.DepositHandler).Methods("POST")
	protected.HandleFunc("/accounts/{id}/withdraw", handler.WithdrawHandler).Methods("POST")
	protected.HandleFunc("/accounts/{id}/transfer", handler.TransferHandler).Methods("POST")
	protected.HandleFunc("/accounts/{id}/predict", handler.PredictAccountBalanceHandler).Methods("GET")
	protected.HandleFunc("/accounts/{id}/cards", handler.GetCardsByAccountHandler).Methods("GET")

	protected.HandleFunc("/cards", handler.CreateCardHandler).Methods("POST")
	protected.HandleFunc("/cards/{id}/verify-cvv", handler.VerifyCVVHandler).Methods("POST")
	protected.HandleFunc("/cards/{id}", handler.DeleteCardHandler).Methods("DELETE")

	protected.HandleFunc("/credits", handler.ApplyForCreditHandler).Methods("POST")
	protected.HandleFunc("/credits", handler.GetUserCreditsHandler).Methods("GET")
	protected.HandleFunc("/credits/{id}/schedule", handler.GetCreditScheduleHandler).Methods("GET")

	protected.HandleFunc("/analytics/income-expense", handler.GetMonthlyIncomeExpenseHandler).Methods("GET")
	protected.HandleFunc("/analytics/credit-load", handler.GetCreditLoadAnalyticsHandler).Methods("GET")
	logrus.Info("Routes registered.")

	serverAddr := fmt.Sprintf(":%d", cfg.ServerPort)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logrus.Infof("Server starting on %s", serverAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.Fatalf("Could not listen on %s: %v", serverAddr, err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logrus.Info("Server shutting down...")
	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelTimeout()

	if err := srv.Shutdown(ctxTimeout); err != nil {
		logrus.Fatalf("Server shutdown failed: %v", err)
	}
	logrus.Info("Server stopped gracefully.")
}
