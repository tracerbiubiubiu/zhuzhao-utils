package validate

import "testing"

// D2-45：LtreeLabel / Identifier 无单测——组织 code 与角色/菜单 code 的
// 字符合约全靠隐式约定，补齐边界用例
func TestLtreeLabel(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"root", true},
		{"tech_center", true},
		{"A1b2", true},
		{"", false},
		{"root.tech", false}, // '.' 是 ltree 分隔符，label 内禁止
		{"tech-center", false},
		{"技术中心", false},
		{"a b", false},
		{"a/b", false},
	}
	for _, c := range cases {
		if got := LtreeLabel(c.in); got != c.want {
			t.Errorf("LtreeLabel(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIdentifier(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"role_a", true},
		{"_private", true},
		{"Role1", true},
		{"", false},
		{"1role", false},  // 首字符须字母/下划线
		{"role-a", false}, // 连字符不支持（与 ltree 白名单对齐）
		{"角色", false},
		{"role.a", false},
	}
	for _, c := range cases {
		if got := Identifier(c.in); got != c.want {
			t.Errorf("Identifier(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
