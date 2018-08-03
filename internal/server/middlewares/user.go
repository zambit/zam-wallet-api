package middlewares

import (
	"context"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/gin-gonic/gin"
	"net/http"
)

var (
	// ErrOnlyMePermitted
	ErrOnlyMePermitted = base.NewErrorsView("").AddField(
		"path", "user_id", `Only "me" value permitted for you`,
	)

	// ErrWrongUserID
	ErrWrongUserID = base.NewErrorsView("").AddField(
		"path", "user_id", "Wrong value",
	)

	// ErrMissingMeAuthInfo
	ErrMissingMeAuthInfo = base.ErrorView{
		Code:    http.StatusInternalServerError,
		Message: "Cannot obtain user auth info",
	}

	// ErrWrongAuthInfo
	ErrWrongAuthInfo = base.ErrorView{
		Code:    http.StatusInternalServerError,
		Message: "Wrong auth info: cannot obtain user ID",
	}
)

const contextUserIDKey = "user_id"

// ContextAuthUserInfoGetter
type ContextAuthUserInfoGetter func(c context.Context) (userID int64, present bool, valid bool)

// UserMiddlewareFactory is factory of middlewares which parses and attaches user ID to an context. Intended to be
// attached on '*/user/:user_id/*' routes, with 'user_id' path parameter. Also it's requires
// 'ContextAuthUserInfoGetter' which is used to extract user auth data.
//
// User ID restored in this ways:
//
// 1. If path parameter user_id equal to 'me', then auth info will be restored using passed ContextAuthUserInfoGetter.
// In case of missing data internal error will be returned.
//
// 2. Else 403 Bad Request will be returned.
func UserMiddlewareFactory(getter ContextAuthUserInfoGetter) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		userIdRaw := c.Param("user_id")
		if userIdRaw == "" {
			err = ErrWrongUserID
			return
		}
		// will be changed in future
		if userIdRaw != "me" {
			err = ErrOnlyMePermitted
			return
		}

		// access user userID
		userID, presented, valid := getter(c)
		if !presented {
			err = ErrMissingMeAuthInfo
			return
		} else if !valid {
			err = ErrWrongAuthInfo
			return
		}

		// attach user id
		c.Set(contextUserIDKey, userID)

		return
	}
}

// GetUserIDFromContext get user ID which was previously attached from context
func GetUserIDFromContext(ctx context.Context) (userID int64, presented bool) {
	userID, presented = ctx.Value(contextUserIDKey).(int64)
	return
}
