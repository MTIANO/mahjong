package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthService struct {
	appID       string
	secret      string
	jwtSecret   string
	expireHours int
	store       *StockStore
}

func NewAuthService(appID, secret, jwtSecret string, expireHours int, store *StockStore) *AuthService {
	return &AuthService{
		appID:       appID,
		secret:      secret,
		jwtSecret:   jwtSecret,
		expireHours: expireHours,
		store:       store,
	}
}

type wxCode2SessionResp struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

func (s *AuthService) Login(code string) (string, int64, error) {
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		s.appID, s.secret, code,
	)

	resp, err := http.Get(url)
	if err != nil {
		return "", 0, fmt.Errorf("request wechat: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("read response: %w", err)
	}

	var wxResp wxCode2SessionResp
	if err := json.Unmarshal(body, &wxResp); err != nil {
		return "", 0, fmt.Errorf("unmarshal: %w", err)
	}
	if wxResp.ErrCode != 0 {
		return "", 0, fmt.Errorf("wechat error [%d]: %s", wxResp.ErrCode, wxResp.ErrMsg)
	}

	user, err := s.store.FindOrCreateUser(wxResp.OpenID)
	if err != nil {
		return "", 0, fmt.Errorf("find or create user: %w", err)
	}

	token, err := s.GenerateToken(user.ID)
	if err != nil {
		return "", 0, fmt.Errorf("generate token: %w", err)
	}

	return token, user.ID, nil
}

func (s *AuthService) GenerateToken(userID int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Duration(s.expireHours) * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *AuthService) ParseToken(tokenStr string) (int64, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid user_id in token")
	}
	return int64(userIDFloat), nil
}
