package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Service behaviors for testing different failure scenarios.
// Dev: "We discovered these patterns after real production incidents!".
type ServiceBehavior int

const (
	BehaviorNormal         ServiceBehavior = iota
	BehaviorSlow                           // Still works but 3x slower (S3 on a bad day)
	BehaviorTimeout                        // Hangs forever (corrupted image in processor)
	BehaviorError                          // Always fails (service down)
	BehaviorTransientError                 // 30% error rate (database under load)
	BehaviorDegraded                       // Works but 2x slower (service degraded)
)

// ModerationService stub.
type ModerationService struct {
	Name            string
	Behavior        ServiceBehavior
	ResponseTime    time.Duration
	FailureCount    int
	currentAttempts int
	mu              sync.Mutex
}

func (m *ModerationService) Moderate(ctx context.Context, p ProfileUpdate) (ProfileUpdate, error) {
	m.mu.Lock()
	m.currentAttempts++
	attempts := m.currentAttempts
	m.mu.Unlock()

	switch m.Behavior {
	case BehaviorNormal:
		select {
		case <-time.After(m.ResponseTime):
			p.ModerationResult = ModerationApproved
			p.ProcessingLog = append(p.ProcessingLog, fmt.Sprintf("%s: approved", m.Name))
			return p, nil
		case <-ctx.Done():
			return p, ctx.Err()
		}

	case BehaviorSlow:
		select {
		case <-time.After(m.ResponseTime * 5):
			p.ModerationResult = ModerationApproved
			p.ProcessingLog = append(p.ProcessingLog, fmt.Sprintf("%s: approved (slow)", m.Name))
			return p, nil
		case <-ctx.Done():
			return p, ctx.Err()
		}

	case BehaviorTimeout:
		<-ctx.Done()
		return p, ctx.Err()

	case BehaviorError:
		return p, fmt.Errorf("%s: service unavailable", m.Name)

	case BehaviorTransientError:
		if attempts <= m.FailureCount {
			return p, fmt.Errorf("%s: transient error %d", m.Name, attempts)
		}
		select {
		case <-time.After(m.ResponseTime):
			p.ModerationResult = ModerationApproved
			p.ProcessingLog = append(p.ProcessingLog, fmt.Sprintf("%s: approved after %d attempts", m.Name, attempts))
			return p, nil
		case <-ctx.Done():
			return p, ctx.Err()
		}
	}

	return p, nil
}

func (m *ModerationService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentAttempts = 0
}

// DatabaseService stub.
type DatabaseService struct {
	Behavior        ServiceBehavior
	ResponseTime    time.Duration
	FailureCount    int
	currentAttempts int
	mu              sync.Mutex
}

func (d *DatabaseService) Save(ctx context.Context, p ProfileUpdate) (ProfileUpdate, error) {
	d.mu.Lock()
	d.currentAttempts++
	attempts := d.currentAttempts
	d.mu.Unlock()

	switch d.Behavior {
	case BehaviorNormal:
		select {
		case <-time.After(d.ResponseTime):
			p.ProcessingLog = append(p.ProcessingLog, "database: saved")
			return p, nil
		case <-ctx.Done():
			return p, ctx.Err()
		}

	case BehaviorTransientError:
		if attempts <= d.FailureCount {
			return p, fmt.Errorf("database: deadlock attempt %d", attempts)
		}
		select {
		case <-time.After(d.ResponseTime):
			p.ProcessingLog = append(p.ProcessingLog, fmt.Sprintf("database: saved after %d attempts", attempts))
			return p, nil
		case <-ctx.Done():
			return p, ctx.Err()
		}

	case BehaviorError:
		return p, errors.New("database: connection failed")

	case BehaviorTimeout:
		<-ctx.Done()
		return p, ctx.Err()
	}

	return p, nil
}

func (d *DatabaseService) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.currentAttempts = 0
}

// CDNService stub (also used for Redis, Varnish).
type CDNService struct {
	Name         string
	Behavior     ServiceBehavior
	ResponseTime time.Duration
}

func (c *CDNService) Invalidate(ctx context.Context, _ string) error {
	switch c.Behavior {
	case BehaviorNormal:
		select {
		case <-time.After(c.ResponseTime):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	case BehaviorError:
		return fmt.Errorf("%s: invalidation failed", c.Name)
	case BehaviorTimeout:
		<-ctx.Done()
		return ctx.Err()
	case BehaviorSlow:
		select {
		case <-time.After(c.ResponseTime * 3):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// NotificationService stub.
type NotificationService struct {
	Name         string
	Behavior     ServiceBehavior
	ResponseTime time.Duration
}

func (n *NotificationService) Send(ctx context.Context, _ string) error {
	switch n.Behavior {
	case BehaviorNormal:
		select {
		case <-time.After(n.ResponseTime):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	case BehaviorError:
		return fmt.Errorf("%s: failed to send", n.Name)
	case BehaviorTimeout:
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

// Global service instances.
var (
	// Moderation services.
	ModerationAPI = &ModerationService{
		Name:         "moderation-api",
		Behavior:     BehaviorNormal,
		ResponseTime: 200 * time.Millisecond,
	}

	ModerationBackup = &ModerationService{
		Name:         "moderation-backup",
		Behavior:     BehaviorNormal,
		ResponseTime: 400 * time.Millisecond,
	}

	// Database.
	Database = &DatabaseService{
		Behavior:     BehaviorNormal,
		ResponseTime: 30 * time.Millisecond,
	}

	// Cache services.
	CDN = &CDNService{
		Name:         "cdn",
		Behavior:     BehaviorNormal,
		ResponseTime: 200 * time.Millisecond,
	}

	Redis = &CDNService{
		Name:         "redis",
		Behavior:     BehaviorNormal,
		ResponseTime: 50 * time.Millisecond,
	}

	Varnish = &CDNService{
		Name:         "varnish",
		Behavior:     BehaviorNormal,
		ResponseTime: 100 * time.Millisecond,
	}

	// Notification services.
	EmailService = &NotificationService{
		Name:         "email",
		Behavior:     BehaviorNormal,
		ResponseTime: 1 * time.Second,
	}

	SMSService = &NotificationService{
		Name:         "sms",
		Behavior:     BehaviorNormal,
		ResponseTime: 800 * time.Millisecond,
	}

	WebhookService = &NotificationService{
		Name:         "webhook",
		Behavior:     BehaviorNormal,
		ResponseTime: 500 * time.Millisecond,
	}
)

// ResetAllServices resets all services to default state.
func ResetAllServices() {
	// Reset attempts.
	ModerationAPI.Reset()
	ModerationBackup.Reset()
	Database.Reset()

	// Reset behaviors.
	ModerationAPI.Behavior = BehaviorNormal
	ModerationBackup.Behavior = BehaviorNormal
	Database.Behavior = BehaviorNormal
	CDN.Behavior = BehaviorNormal
	Redis.Behavior = BehaviorNormal
	Varnish.Behavior = BehaviorNormal
	EmailService.Behavior = BehaviorNormal
	SMSService.Behavior = BehaviorNormal
	WebhookService.Behavior = BehaviorNormal
}
