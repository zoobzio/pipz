package main

import "time"

// Constants for all processor and connector names.
const (
	// Processors.
	ProcessorValidateImage     = "validate_image"
	ProcessorProcessImage      = "process_image"
	ProcessorModerateContent   = "moderate_content"
	ProcessorModerateBackup    = "moderate_backup"
	ProcessorSkipModeration    = "skip_moderation"
	ProcessorUpdateDatabase    = "update_database"
	ProcessorInvalidateCDN     = "invalidate_cdn"
	ProcessorInvalidateRedis   = "invalidate_redis"
	ProcessorInvalidateVarnish = "invalidate_varnish"
	ProcessorSendEmail         = "send_email"
	ProcessorSendSMS           = "send_sms"
	ProcessorSendSMSPremium    = "send_sms_premium"
	ProcessorPostWebhook       = "post_webhook"
	ProcessorCleanupImages     = "cleanup_images"
	ProcessorLogCacheErrors    = "log_cache_errors"

	// Connectors.
	ConnectorImageProcessing   = "image-processing"
	ConnectorModeration        = "moderation-fallback"
	ConnectorDatabase          = "database-retry"
	ConnectorCacheInvalidation = "cache-invalidation"
	ConnectorNotifications     = "notifications"
	PipelineProfileUpdate      = "profile-update"
	PipelineProfileUpdateSafe  = "profile-update-safe"
)

// ProfileUpdate represents the data flowing through the pipeline.
type ProfileUpdate struct { //nolint:govet // demo struct
	UserID           string
	Photo            []byte
	ProcessedImages  ProcessedImages
	ModerationResult ModerationResult
	Timestamp        time.Time
	RequestID        string
	ProcessingLog    []string // For tracking what happened
	IsPremium        bool     // For conditional processing examples
}

// Clone implements the Cloner interface for use with Concurrent.
func (p ProfileUpdate) Clone() ProfileUpdate {
	// Deep copy slices.
	newPhoto := make([]byte, len(p.Photo))
	copy(newPhoto, p.Photo)

	newLog := make([]string, len(p.ProcessingLog))
	copy(newLog, p.ProcessingLog)

	// ProcessedImages contains only byte slices, safe to copy.
	newProcessed := ProcessedImages{}
	if p.ProcessedImages.Thumbnail != nil {
		newProcessed.Thumbnail = make([]byte, len(p.ProcessedImages.Thumbnail))
		copy(newProcessed.Thumbnail, p.ProcessedImages.Thumbnail)
	}
	if p.ProcessedImages.Medium != nil {
		newProcessed.Medium = make([]byte, len(p.ProcessedImages.Medium))
		copy(newProcessed.Medium, p.ProcessedImages.Medium)
	}
	if p.ProcessedImages.Large != nil {
		newProcessed.Large = make([]byte, len(p.ProcessedImages.Large))
		copy(newProcessed.Large, p.ProcessedImages.Large)
	}

	return ProfileUpdate{
		UserID:           p.UserID,
		Photo:            newPhoto,
		ProcessedImages:  newProcessed,
		ModerationResult: p.ModerationResult,
		Timestamp:        p.Timestamp,
		RequestID:        p.RequestID,
		ProcessingLog:    newLog,
		IsPremium:        p.IsPremium,
	}
}

type ProcessedImages struct {
	Thumbnail []byte
	Medium    []byte
	Large     []byte
}

type ModerationResult int

const (
	ModerationPending ModerationResult = iota
	ModerationApproved
	ModerationFlagged
	ModerationSkipped
)

func (m ModerationResult) String() string {
	switch m {
	case ModerationPending:
		return "Pending"
	case ModerationApproved:
		return "Approved"
	case ModerationFlagged:
		return "Flagged"
	case ModerationSkipped:
		return "Skipped"
	default:
		return "Unknown"
	}
}
