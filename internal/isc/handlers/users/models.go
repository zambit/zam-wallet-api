package users

// CreatedEvent select only user id which is interesting to us
type CreatedEvent struct {
	UserPhone string `json:"user_phone"`
}
