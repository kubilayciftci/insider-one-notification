package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
	"github.com/kubilayciftci/insider-one-notification/internal/core/ports"
	"github.com/kubilayciftci/insider-one-notification/internal/core/service"
	"go.opentelemetry.io/otel/trace"
)

type Handler struct {
	svc    *service.NotificationService
	logger *slog.Logger
}

func NewHandler(svc *service.NotificationService, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/notifications", h.CreateNotification)
	r.Post("/notifications/batch", h.CreateBatch)
	r.Get("/notifications/{id}", h.GetNotification)
	r.Get("/notifications/batch/{batchId}", h.GetByBatchID)
	r.Delete("/notifications/{id}", h.CancelNotification)
	r.Get("/notifications", h.ListNotifications)
	r.Get("/health", h.HealthCheck)
	return r
}

func (h *Handler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var req CreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	channel, err := domain.ParseChannel(req.Channel)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error(), "INVALID_CHANNEL")
		return
	}

	priority := domain.PriorityNormal
	if req.Priority != "" {
		priority, err = domain.ParsePriority(req.Priority)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error(), "INVALID_PRIORITY")
			return
		}
	}

	n, err := h.svc.Create(r.Context(), service.CreateRequest{
		Recipient:      req.Recipient,
		Channel:        channel,
		Content:        req.Content,
		Priority:       priority,
		IdempotencyKey: req.IdempotencyKey,
		Payload:        req.Payload,
	})
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusAccepted, toNotificationResponse(n))
}

func (h *Handler) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var req CreateBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	requests := make([]service.CreateRequest, 0, len(req.Notifications))
	for _, nr := range req.Notifications {
		channel, err := domain.ParseChannel(nr.Channel)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error(), "INVALID_CHANNEL")
			return
		}
		priority := domain.PriorityNormal
		if nr.Priority != "" {
			var pErr error
			priority, pErr = domain.ParsePriority(nr.Priority)
			if pErr != nil {
				writeError(w, r, http.StatusBadRequest, pErr.Error(), "INVALID_PRIORITY")
				return
			}
		}
		requests = append(requests, service.CreateRequest{
			Recipient:      nr.Recipient,
			Channel:        channel,
			Content:        nr.Content,
			Priority:       priority,
			IdempotencyKey: nr.IdempotencyKey,
			Payload:        nr.Payload,
		})
	}

	batchID, notifications, err := h.svc.CreateBatch(r.Context(), requests)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusAccepted, BatchResponse{
		BatchID:       batchID,
		Notifications: toNotificationResponses(notifications),
		Count:         len(notifications),
	})
}

func (h *Handler) GetNotification(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid notification ID", "INVALID_ID")
		return
	}

	n, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toNotificationResponse(n))
}

func (h *Handler) GetByBatchID(w http.ResponseWriter, r *http.Request) {
	batchID, err := uuid.Parse(chi.URLParam(r, "batchId"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid batch ID", "INVALID_ID")
		return
	}

	notifications, err := h.svc.GetByBatchID(r.Context(), batchID)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, BatchResponse{
		BatchID:       batchID,
		Notifications: toNotificationResponses(notifications),
		Count:         len(notifications),
	})
}

func (h *Handler) CancelNotification(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid notification ID", "INVALID_ID")
		return
	}

	if err := h.svc.Cancel(r.Context(), id); err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	filter := ports.ListFilter{
		Page:     queryInt(r, "page", 1),
		PageSize: queryInt(r, "page_size", 20),
	}

	if s := r.URL.Query().Get("status"); s != "" {
		status := domain.Status(s)
		filter.Status = &status
	}
	if c := r.URL.Query().Get("channel"); c != "" {
		channel := domain.Channel(c)
		filter.Channel = &channel
	}
	if from := r.URL.Query().Get("from_date"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filter.FromDate = &t
		}
	}
	if to := r.URL.Query().Get("to_date"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filter.ToDate = &t
		}
	}

	notifications, total, err := h.svc.List(r.Context(), filter)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, ListResponse{
		Notifications: toNotificationResponses(notifications),
		Total:         total,
		Page:          filter.Page,
		PageSize:      filter.PageSize,
	})
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "notification not found", "NOT_FOUND")
	case errors.Is(err, domain.ErrNotCancellable):
		writeError(w, r, http.StatusConflict, err.Error(), "NOT_CANCELLABLE")
	case errors.Is(err, domain.ErrDuplicateKey):
		writeError(w, r, http.StatusConflict, "duplicate idempotency key", "DUPLICATE_KEY")
	case errors.Is(err, domain.ErrBatchTooLarge):
		writeError(w, r, http.StatusBadRequest, err.Error(), "BATCH_TOO_LARGE")
	case errors.Is(err, domain.ErrEmptyRecipient),
		errors.Is(err, domain.ErrEmptyContent),
		errors.Is(err, domain.ErrContentTooLong),
		errors.Is(err, domain.ErrInvalidChannel),
		errors.Is(err, domain.ErrInvalidPriority):
		writeError(w, r, http.StatusBadRequest, err.Error(), "VALIDATION_ERROR")
	default:
		writeError(w, r, http.StatusInternalServerError, "internal server error", "INTERNAL_ERROR")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, msg, code string) {
	traceID := ""
	if span := trace.SpanFromContext(r.Context()); span.SpanContext().HasTraceID() {
		traceID = span.SpanContext().TraceID().String()
	}
	writeJSON(w, status, ErrorResponse{Error: msg, Code: code, TraceID: traceID})
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(v)
	if err != nil || i <= 0 {
		return defaultVal
	}
	return i
}
