package harness

// 权限动作。
const (
	Allow = "allow"
	Ask   = "ask"
	Deny  = "deny"
)

// Perms 是按工具敏感度的权限策略:sensitivity(read/write/exec)-> allow|ask|deny。
type Perms map[string]string

// Action 返回该敏感度的动作;未配置一律按 ask(保守:宁可多问一次)。
func (p Perms) Action(sensitivity string) string {
	if a, ok := p[sensitivity]; ok && a != "" {
		return a
	}
	return Ask
}
