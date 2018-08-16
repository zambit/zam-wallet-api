package isc

import "git.zam.io/wallet-backend/common/pkg/types/decimal"

// UserStatRequest used to parse incoming user statistic request
type UserStatRequest struct {
	UserPhone string `form:"user_phone" validate:"required,phone"`
	Convert   string `form:"convert"`
}

func DefaultUserStatRequest() UserStatRequest {
	return UserStatRequest{
		Convert: "usd",
	}
}

// UserStatsResponseView represents
type UserStatsResponseView struct {
	Count        int                      `json:"count"`
	TotalBalance map[string]*decimal.View `json:"total_balance"`
}
