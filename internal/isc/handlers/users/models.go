package users

// CreatedEvent select only user id which is interesting to us
type CreatedEvent struct {
	UserID int64 `json:"user_id,string"`
}
