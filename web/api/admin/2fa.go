package admin

import (
	"image/png"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/web/api"
	"github.com/pquerna/otp/totp"
)

func Generate2FA(c *gin.Context) {
	if !allowEnrollment(c) {
		return
	}
	secret, img, err := accounts.Generate2Fa()
	if err != nil {
		api.RespondError(c, 500, "Failed to generate 2FA: "+err.Error())
		return
	}
	setEnrollmentCookie(c, secret, 300)
	c.Header("Content-Type", "image/png")
	c.Writer.WriteHeader(200)
	png.Encode(c.Writer, img)
}

func Enable2FA(c *gin.Context) {
	if !allowEnrollment(c) {
		return
	}
	uuid, _ := c.Get("uuid")
	secret, _ := c.Cookie("2fa_secret")
	code := c.Query("code")
	if secret == "" || uuid == nil || code == "" {
		api.RespondError(c, 400, "2FA secret or code not provided")
		return
	}
	if !totp.Validate(code, secret) {
		api.RespondError(c, 400, "Invalid 2FA code")
		return
	}
	err := accounts.Enable2Fa(uuid.(string), secret)
	if err != nil {
		api.RespondError(c, 500, "Failed to enable 2FA: "+err.Error())
		return
	}
	setEnrollmentCookie(c, "", -1)

	api.RespondSuccess(c, "2FA enabled successfully")
}

// An existing factor must be explicitly disabled with its current code first.
func allowEnrollment(c *gin.Context) bool {
	uuid, _ := c.Get("uuid")
	id, _ := uuid.(string)
	user, err := accounts.GetUserByUUID(id)
	if err != nil || user.TwoFactor != "" {
		api.RespondError(c, http.StatusForbidden, "Disable the existing second factor before enrolling a new one")
		return false
	}
	return true
}

func setEnrollmentCookie(c *gin.Context, secret string, age int) {
	http.SetCookie(c.Writer, &http.Cookie{Name: "2fa_secret", Value: secret,
		Path: "/", MaxAge: age, HttpOnly: true, Secure: utils.GetScheme(c) == "https", SameSite: http.SameSiteStrictMode})
}

func Disable2FA(c *gin.Context) {
	uuid, _ := c.Get("uuid")
	err := accounts.Disable2Fa(uuid.(string))
	if err != nil {
		api.RespondError(c, 500, "Failed to disable 2FA: "+err.Error())
		return
	}
	api.RespondSuccess(c, "")
}
