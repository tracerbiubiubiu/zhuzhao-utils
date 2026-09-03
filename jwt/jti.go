package jwt

import "github.com/google/uuid"

// generateJTI 生成 token 唯一标识
func generateJTI() string {
	return uuid.New().String()
}
