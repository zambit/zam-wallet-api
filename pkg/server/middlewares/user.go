package middlewares

import (
	"context"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/gin-gonic/gin"
	"net/http"
)

var (
	// ErrOnlyMePermitted
	ErrOnlyMePermitted = base.NewFieldErr("path", "user_phone", `only "me" value permitted for you`)

	// ErrWrongUserPhone
	ErrWrongUserPhone = base.NewFieldErr("path", "user_phone", "wrong value")

	// ErrMissingMeAuthInfo
	ErrMissingMeAuthInfo = base.ErrorView{
		Code:    http.StatusInternalServerError,
		Message: "cannot obtain user auth info",
	}

	// ErrWrongAuthInfo
	ErrWrongAuthInfo = base.ErrorView{
		Code:    http.StatusInternalServerError,
		Message: "wrong auth info: cannot obtain user phone",
	}

	// ErrUserMiddlewareMissing returned from GetUserPhoneFromCtxE to indicate that user middleware is missing
	ErrUserMiddlewareMissing = base.ErrorView{
		Code:    http.StatusInternalServerError,
		Message: "user middleware is missing",
	}

	// ErrUserMiddlewareMissing returned when some critical errors occurs
	ErrUserMiddlewareBadBehaviour = base.ErrorView{
		Code:    http.StatusInternalServerError,
		Message: "user middleware bad behaviour",
	}
)

const (
	contextUserPhoneKey     = "user_phone"
	contextUserMwAppliedKey = "user_mw_applied"
)

// ContextAuthUserInfoGetter
type ContextAuthUserInfoGetter func(c context.Context) (userPhone string, present bool, valid bool)

// UserMiddlewareFactory is factory of middlewares which parses and attaches user ID to an context. Intended to be
// attached on '*/user/:user_phone/*' routes, with 'user_phone' path parameter. Also it's requires
// 'ContextAuthUserInfoGetter' which is used to extract user auth data.
//
// User ID restored in this ways:
//
// 1. If path parameter user_phone equal to 'me', then auth info will be restored using passed ContextAuthUserInfoGetter.
// In case of missing data internal error will be returned.
//
// 2. Else 403 Bad Request will be returned.
func UserMiddlewareFactory(getter ContextAuthUserInfoGetter) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		userIdRaw := c.Param("user_phone")
		if userIdRaw == "" {
			err = ErrWrongUserPhone
			return
		}
		// will be changed in future
		if userIdRaw != "me" {
			err = ErrOnlyMePermitted
			return
		}

		// access user userPhone
		userPhone, presented, valid := getter(c)
		if !presented {
			err = ErrMissingMeAuthInfo
			return
		} else if !valid {
			err = ErrWrongAuthInfo
			return
		}

		// attach user id
		c.Set(contextUserPhoneKey, userPhone)
		// attach user middleware applied key
		c.Set(contextUserMwAppliedKey, true)

		return
	}
}

// GetUserPhoneFromContext get user ID which was previously attached from context
func GetUserPhoneFromContext(ctx context.Context) (userPhone string, presented bool) {
	userPhone, presented = ctx.Value(contextUserPhoneKey).(string)
	return
}

// GetUserPhoneFromCtxE same as GetUserPhoneFromContext but returns middleware missing error
func GetUserPhoneFromCtxE(ctx context.Context) (userPhone string, err error) {
	if ctx.Value(contextUserMwAppliedKey) == nil {
		err = ErrUserMiddlewareMissing
		return
	}

	userPhone, valid := ctx.Value(contextUserPhoneKey).(string)
	if !valid {
		err = ErrUserMiddlewareBadBehaviour
	}
	return
}
