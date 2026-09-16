package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/tracerbiubiubiu/zhuzhao-utils/aksk"
)

// AKSKFail 分档映射：状态码按可修正性、码值引 errcode 常量、文案按失败原因。
func TestAKSKFail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    int
		wantMsgPart string
	}{
		{"body too large", aksk.ErrBodyTooLarge, http.StatusRequestEntityTooLarge, 10001, "读体上限"},
		{"body read", aksk.ErrBodyRead, http.StatusBadRequest, 10001, "读取失败"},
		{"missing header", aksk.ErrMissingHeader, http.StatusUnauthorized, 10002, "缺少 Authorization"},
		{"bad header", aksk.ErrBadHeader, http.StatusUnauthorized, 10002, "格式错误"},
		{"unknown credential", fmt.Errorf("%w: zhuzhao", aksk.ErrUnknownCredential), http.StatusUnauthorized, 10002, "未登记或已停用"},
		{"expired", aksk.ErrExpired, http.StatusUnauthorized, 10002, "时间窗口"},
		{"bad signature", aksk.ErrBadSignature, http.StatusUnauthorized, 10002, "签名校验失败"},
		{"unknown error", errors.New("boom"), http.StatusUnauthorized, 10002, "鉴权失败"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/x", nil)
			AKSKFail()(c, tc.err)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			var body struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Detail  string `json:"detail"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("body: %v (%s)", err, w.Body.String())
			}
			if body.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d", body.Code, tc.wantCode)
			}
			if !strings.Contains(body.Message, tc.wantMsgPart) {
				t.Fatalf("message = %q, want contains %q", body.Message, tc.wantMsgPart)
			}
			if body.Detail != "" {
				t.Fatalf("envelope must not carry detail, got %q", body.Detail)
			}
		})
	}
}
