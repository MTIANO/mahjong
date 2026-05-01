package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/service"
)

func TestRecognizeHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	vision := service.NewStubVisionService()
	h := NewRecognizeHandler(vision)
	r.POST("/api/v1/recognize", h.Handle)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("image", "test.jpg")
	part.Write([]byte("fake image data"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/recognize", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp RecognizeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Tiles) == 0 {
		t.Error("expected tiles in response")
	}
}

func TestRecognizeHandler_NoImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	vision := service.NewStubVisionService()
	h := NewRecognizeHandler(vision)
	r.POST("/api/v1/recognize", h.Handle)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/recognize", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
