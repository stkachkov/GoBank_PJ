package service

import (
	"GoBank_PJ/bank-service/internal/models"
	"GoBank_PJ/bank-service/internal/repository/postgres"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	CheckInterval         = 12 * time.Hour
	OverdueFinePercentage = 0.10
)

type SchedulerService struct {
	creditService       *CreditService
	accountService      *AccountService
	userRepo            postgres.UserRepository
	notificationService *NotificationService
	ticker              *time.Ticker
	quit                chan struct{}
}

func NewSchedulerService(creditService *CreditService, accountService *AccountService, userRepo postgres.UserRepository, notificationService *NotificationService) *SchedulerService {
	return &SchedulerService{
		creditService:       creditService,
		accountService:      accountService,
		userRepo:            userRepo,
		notificationService: notificationService,
		quit:                make(chan struct{}),
	}
}

func (s *SchedulerService) Start(ctx context.Context) {
	s.ticker = time.NewTicker(CheckInterval)
	logrus.Infof("Scheduler started, checking for overdue payments every %s", CheckInterval)

	go func() {
		for {
			select {
			case <-s.ticker.C:
				logrus.Info("Scheduler: Checking for overdue credit payments...")
				s.processOverduePayments(ctx)
			case <-s.quit:
				s.ticker.Stop()
				logrus.Info("Scheduler stopped.")
				return
			}
		}
	}()
}

func (s *SchedulerService) Stop() {
	close(s.quit)
}

func (s *SchedulerService) processOverduePayments(ctx context.Context) {
	allCredits, err := s.creditService.creditRepo.GetCreditsByUserID(ctx, uuid.Nil)
	if err != nil {
		logrus.Errorf("Scheduler: Error fetching all credits: %v", err)
		return
	}

	for _, credit := range allCredits {
		if credit.Status == "active" || credit.Status == "overdue" {
			schedules, err := s.creditService.GetPaymentSchedule(ctx, credit.ID)
			if err != nil {
				logrus.Errorf("Scheduler: Error getting payment schedule for credit %s: %v", credit.ID, err)
				continue
			}

			for _, schedule := range schedules {
				if !schedule.IsPaid && schedule.DueDate.Before(time.Now()) {
					logrus.Infof("Scheduler: Processing overdue payment for credit %s, schedule %s", credit.ID, schedule.ID)

					err := s.creditService.ProcessMonthlyPayment(ctx, credit.ID, credit.AccountID, schedule.AmountDue)
					if err != nil {
						if errors.Is(err, ErrInsufficientFunds) {
							logrus.Warnf("Scheduler: Insufficient funds for credit %s, account %s. Applying fine.", credit.ID, credit.AccountID)

							schedule.OverdueFines += schedule.AmountDue * OverdueFinePercentage

							if err := s.creditService.paymentScheduleRepo.UpdatePaymentSchedule(ctx, schedule); err != nil {
								logrus.Errorf("Scheduler: Failed to update schedule %s with fine: %v", schedule.ID, err)
							} else {
								go func(c *models.Credit, pSchedule *models.PaymentSchedule) {
									user, err := s.userRepo.FindUserByID(context.Background(), c.UserID.String())
									if err != nil {
										logrus.Errorf("Scheduler: failed to find user %s for fine notification: %v", c.UserID, err)
										return
									}
									subject := "Начисление штрафа за просрочку платежа"
									body := fmt.Sprintf("Здравствуйте, %s! На вашем счете недостаточно средств для списания платежа по кредиту. Начислен штраф в размере %.2f.", user.Username, pSchedule.OverdueFines)
									if err := s.notificationService.SendEmail(context.Background(), user.Email, subject, body); err != nil {
										logrus.Errorf("Scheduler: failed to send fine notification to user %s: %v", c.UserID, err)
									}
								}(credit, schedule)
							}

							if credit.Status == "active" {
								credit.Status = "overdue"
								if err := s.creditService.creditRepo.UpdateCredit(ctx, credit); err != nil {
									logrus.Errorf("Scheduler: Failed to update credit %s status to overdue: %v", credit.ID, err)
								}
							}
						} else {
							logrus.Errorf("Scheduler: Error processing payment for credit %s, schedule %s: %v", credit.ID, schedule.ID, err)
						}
					} else {
						logrus.Infof("Scheduler: Successfully processed payment for credit %s, schedule %s.", credit.ID, schedule.ID)
						go func(c *models.Credit, pSchedule *models.PaymentSchedule) {
							user, err := s.userRepo.FindUserByID(context.Background(), c.UserID.String())
							if err != nil {
								logrus.Errorf("Scheduler: failed to find user %s for success notification: %v", c.UserID, err)
								return
							}
							subject := "Успешное списание по кредиту"
							body := fmt.Sprintf("Здравствуйте, %s! Платеж по вашему кредиту на сумму %.2f был успешно списан.", user.Username, pSchedule.AmountDue)
							if err := s.notificationService.SendEmail(context.Background(), user.Email, subject, body); err != nil {
								logrus.Errorf("Scheduler: failed to send success notification to user %s: %v", c.UserID, err)
							}
						}(credit, schedule)
					}
				}
			}
		}
	}
}
