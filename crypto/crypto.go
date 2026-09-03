package crypto

import "golang.org/x/crypto/bcrypt"

// dummyHash 固定的 bcrypt 哈希（"dummy-password-for-timing" 的 cost=12 散列）。
// 登录时工号不存在分支用它做一次等价比对，拉平与「工号存在但密码错」分支的
// 时延差，防定时侧信道枚举有效工号（B4-1 / R1-AUTH-03）。
const dummyHash = "$2a$12$qH1GXSnu9xI6yF/HLIPSouhrEOYcu5RATjybL.FvkGPE/tiGeVL02"

// HashPassword 密码哈希
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

// CheckPassword 校验密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// CheckDummyPassword 对固定哈希做一次比对（恒定失败），用于拉平登录时延。
// 结果恒为 false，仅为时延对齐，勿用于真实校验。
func CheckDummyPassword(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
}
