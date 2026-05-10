package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type loginRequest struct {
	Code string `json:"code" binding:"required"`
}

type loginResponse struct {
	Token  string `json:"token"`
	UserID int64  `json:"user_id"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	token, userID, err := h.auth.Login(req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, loginResponse{Token: token, UserID: userID})
}
