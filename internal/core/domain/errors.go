package domain

import "errors"

var (
	ErrNotFound        = errors.New("notification not found")
	ErrInvalidChannel  = errors.New("invalid channel: must be sms, email, or push")
	ErrInvalidPriority = errors.New("invalid priority: must be high, normal, or low")
	ErrEmptyRecipient  = errors.New("recipient is required")
	ErrEmptyContent    = errors.New("content is required")
	ErrContentTooLong  = errors.New("content exceeds maximum length for channel")
	ErrNotCancellable  = errors.New("notification cannot be cancelled in current status")
	ErrBatchTooLarge   = errors.New("batch size exceeds maximum of 1000")
	ErrDuplicateKey    = errors.New("idempotency key already exists")
)
