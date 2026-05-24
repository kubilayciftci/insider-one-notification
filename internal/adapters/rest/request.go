package rest

type CreateNotificationRequest struct {
	Recipient      string         `json:"recipient"`
	Channel        string         `json:"channel"`
	Content        string         `json:"content"`
	Priority       string         `json:"priority"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
}

type CreateBatchRequest struct {
	Notifications []CreateNotificationRequest `json:"notifications"`
}

type ListQueryParams struct {
	Status   string `json:"status"`
	Channel  string `json:"channel"`
	FromDate string `json:"from_date"`
	ToDate   string `json:"to_date"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}
