package handler

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"main/repository"
	"main/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
)

type Enable2FAResponse struct {
	Secret        string   `json:"secret"`
	QRCode        string   `json:"qr_code"`
	RecoveryCodes []string `json:"recovery_codes,omitempty"`
}

func Enable2FAHandler(c *gin.Context) {
	var req struct {
		Secret string `json:"secret" binding:"required"`
		Code   string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request")
		return
	}

	userID, _ := c.Get("user_id")
	userRepo := repository.GetUserRepo(utils.MongoClient)

	// Check if 2FA is already enabled
	user, err := userRepo.FindUser(userID.(string))
	if err != nil {
		utils.InternalError(c, "Failed to fetch user")
		return
	}

	if user.TwoFactorEnabled {
		utils.BadRequest(c, "2FA is already enabled")
		return
	}

	// Verify the code is valid
	valid := totp.Validate(req.Code, req.Secret)
	if !valid {
		utils.BadRequest(c, "Invalid 2FA code")
		return
	}

	// Generate recovery codes
	recoveryCodes, err := utils.GenerateRecoveryCodes()
	if err != nil {
		utils.InternalError(c, "Failed to generate recovery codes")
		return
	}

	// Hash recovery codes for storage
	hashedCodes := utils.HashRecoveryCodes(recoveryCodes)

	// Enable 2FA with recovery codes
	err = userRepo.Enable2FAWithRecoveryCodes(userID.(string), req.Secret, hashedCodes)
	if err != nil {
		utils.InternalError(c, "Failed to enable 2FA")
		return
	}

	utils.Success(c, gin.H{
		"message":        "2FA enabled successfully",
		"recovery_codes": recoveryCodes, // Send plain text codes to user
		"warning":        "Save these recovery codes securely. They will not be shown again.",
	})
}

func UseRecoveryCodeHandler(c *gin.Context) {
	var req struct {
		RecoveryCode string `json:"recovery_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request")
		return
	}

	userID, _ := c.Get("user_id")
	userRepo := repository.GetUserRepo(utils.MongoClient)
	user, err := userRepo.FindUser(userID.(string))
	if err != nil {
		utils.InternalError(c, "Failed to fetch user")
		return
	}

	// Format the recovery code
	code := strings.ToUpper(strings.ReplaceAll(req.RecoveryCode, "-", ""))
	hashedCode := utils.HashString(code)

	// Check if the code exists and remove it
	found := false
	newCodes := make([]string, 0)
	for _, storedCode := range user.RecoveryCodes {
		if storedCode == hashedCode {
			found = true
		} else {
			newCodes = append(newCodes, storedCode)
		}
	}

	if !found {
		utils.Unauthorized(c, "Invalid recovery code")
		return
	}

	// Update user with remaining recovery codes
	err = userRepo.UpdateRecoveryCodes(userID.(string), newCodes)
	if err != nil {
		utils.InternalError(c, "Failed to update recovery codes")
		return
	}

	utils.Success(c, gin.H{
		"message":         "Recovery code accepted",
		"remaining_codes": len(newCodes),
		"warning":         "Please set up a new authenticator app as soon as possible",
	})
}

// Generate2FASecretHandler generates a new 2FA secret and QR code
func Generate2FASecretHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "Missing or invalid token")
		return
	}

	userRepo := repository.GetUserRepo(utils.MongoClient)
	user, err := userRepo.FindUser(userID.(string))
	if err != nil {
		utils.InternalError(c, "Failed to fetch user")
		return
	}

	if user.TwoFactorEnabled {
		utils.BadRequest(c, "2FA is already enabled")
		return
	}

	// Generate new TOTP key
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "ToNotes",
		AccountName: user.Email,
	})
	if err != nil {
		utils.InternalError(c, "Failed to generate 2FA secret")
		return
	}

	// Generate QR code
	var buf bytes.Buffer
	img, err := key.Image(200, 200)
	if err != nil {
		utils.InternalError(c, "Failed to generate QR code")
		return
	}

	if err := png.Encode(&buf, img); err != nil {
		utils.InternalError(c, "Failed to encode QR code")
		return
	}

	qrCode := base64.StdEncoding.EncodeToString(buf.Bytes())

	utils.Success(c, Enable2FAResponse{
		Secret: key.Secret(),
		QRCode: "data:image/png;base64," + qrCode,
	})
}

// Verify2FAHandler verifies a 2FA code
func Verify2FAHandler(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request")
		return
	}

	userID, _ := c.Get("user_id")
	userRepo := repository.GetUserRepo(utils.MongoClient)
	user, err := userRepo.FindUser(userID.(string))
	if err != nil {
		utils.InternalError(c, "Failed to fetch user")
		return
	}

	if !user.TwoFactorEnabled {
		utils.BadRequest(c, "2FA is not enabled")
		return
	}

	valid := totp.Validate(req.Code, user.TwoFactorSecret)
	if !valid {
		utils.Unauthorized(c, "Invalid 2FA code")
		return
	}

	utils.Success(c, gin.H{"message": "2FA code valid"})
}

// Disable2FAHandler disables 2FA for the user
func Disable2FAHandler(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request")
		return
	}

	userID, _ := c.Get("user_id")
	userRepo := repository.GetUserRepo(utils.MongoClient)
	user, err := userRepo.FindUser(userID.(string))
	if err != nil {
		utils.InternalError(c, "Failed to fetch user")
		return
	}

	if !user.TwoFactorEnabled {
		utils.BadRequest(c, "2FA is not enabled")
		return
	}

	// Verify the code before disabling
	valid := totp.Validate(req.Code, user.TwoFactorSecret)
	if !valid {
		utils.Unauthorized(c, "Invalid 2FA code")
		return
	}

	// Disable 2FA
	err = userRepo.Disable2FA(userID.(string))
	if err != nil {
		utils.InternalError(c, "Failed to disable 2FA")
		return
	}

	utils.Success(c, gin.H{"message": "2FA disabled successfully"})
}
