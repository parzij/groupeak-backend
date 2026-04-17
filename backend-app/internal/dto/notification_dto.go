package dto

type ReadNotificationRequest struct {
	NotificationIDs []int64 `json:"notification_ids"`
}
