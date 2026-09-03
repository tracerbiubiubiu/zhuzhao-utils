package validate

import "unicode"

// LtreeLabel 校验 ltree 标签（组织 code）：仅 [A-Za-z0-9_]
func LtreeLabel(code string) bool {
	if code == "" {
		return false
	}
	for _, r := range code {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

// Identifier 校验业务 code（角色/菜单）：字母数字下划线
func Identifier(code string) bool {
	if code == "" {
		return false
	}
	for i, r := range code {
		if i == 0 && !unicode.IsLetter(r) && r != '_' {
			return false
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}
