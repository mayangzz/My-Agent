package memstore

import (
	"context"
	"encoding/json"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mayangzz/My-Agent/harness"
)

// Layered 是分层记忆:把对话按天归档,每天滚动出一份 ≤N 字的摘要,
// 召回时按"新近度权重 + token 预算"把近若干天的每日摘要 + 当天原始对话拼成上下文。
// 它本身实现 harness.Memory,内部包着任意底层后端(filewiki/postgres/redis)——
// 短期(当天原始)和长期(每日摘要)共用同一套抽象接口,跟后端无关。
//
// 底层 key 约定(底层后端把它当普通 session 名):
//   <base>/<YYYY-MM-DD>/raw      当天原始消息
//   <base>/<YYYY-MM-DD>/digest   当天摘要(单条)
//   <base>/_index                出现过哪些天(JSON 数组,用于保留期清理)
type Layered struct {
	base            harness.Memory
	retentionDays   int
	summaryMaxChars int
	recallBudget    int // 召回每日摘要的字符预算(近的优先,装不下丢老的)
	// summarize 把当天对话渲染文本总结成一段摘要;nil 表示关掉每日摘要。
	summarize func(ctx context.Context, conversation string) (string, error)
	now       func() string // 返回今天 "2006-01-02";可注入便于测试
	mu        sync.Mutex
}

// NewLayered 包住 base 后端。retentionDays<=0 时退化为"只按天存、不清理"。
func NewLayered(base harness.Memory, retentionDays, summaryMaxChars, recallBudget int,
	summarize func(context.Context, string) (string, error)) *Layered {
	return &Layered{
		base:            base,
		retentionDays:   retentionDays,
		summaryMaxChars: summaryMaxChars,
		recallBudget:    recallBudget,
		summarize:       summarize,
		now:             func() string { return time.Now().Format("2006-01-02") },
	}
}

func dayKey(base, date, kind string) string { return base + "/" + date + "/" + kind }
func dayIndexKey(base string) string        { return base + "/_index" }

func (l *Layered) Append(ctx context.Context, base string, m harness.Message) error {
	today := l.now()
	if err := l.base.Append(ctx, dayKey(base, today, "raw"), m); err != nil {
		return err
	}
	l.ensureIndexed(ctx, base, today)
	// 一轮的最终答案(assistant 且不再要工具)落地后,异步刷新当天摘要。
	if l.summarize != nil && m.Role == "assistant" && len(m.ToolCalls) == 0 && strings.TrimSpace(m.Content) != "" {
		go l.refreshDigest(base, today)
	}
	return nil
}

func (l *Layered) Load(ctx context.Context, base string) ([]harness.Message, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	today := l.now()

	kept := l.pruneLocked(ctx, base, today) // 清理超过保留期的天

	// 收集过去几天(不含今天)的摘要
	type dayDigest struct{ date, text string }
	var past []dayDigest
	for _, d := range kept {
		if d == today {
			continue
		}
		dm, _ := l.base.Load(ctx, dayKey(base, d, "digest"))
		if len(dm) > 0 && dm[len(dm)-1].Content != "" {
			past = append(past, dayDigest{d, dm[len(dm)-1].Content})
		}
	}
	sort.Slice(past, func(i, j int) bool { return past[i].date < past[j].date })

	// 按新近度选:从最新往回装,装满预算为止;再按时间正序呈现(近的靠近 prompt)。
	var selected []dayDigest
	used := 0
	for i := len(past) - 1; i >= 0; i-- {
		c := len(past[i].text)
		if used+c > l.recallBudget && len(selected) > 0 {
			break
		}
		selected = append(selected, past[i])
		used += c
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].date < selected[j].date })

	var out []harness.Message
	if len(selected) > 0 {
		var sb strings.Builder
		sb.WriteString("以下是你与用户最近若干天的记忆摘要(越靠后越近、越重要):\n")
		for _, s := range selected {
			sb.WriteString("\n【" + s.date + "】\n" + s.text + "\n")
		}
		out = append(out, harness.Message{Role: "system", Content: sb.String()})
	}

	raw, err := l.base.Load(ctx, dayKey(base, today, "raw"))
	if err != nil {
		return nil, err
	}
	return append(out, raw...), nil
}

// Reset 清当天的工作记忆(原始 + 摘要),保留更早的每日摘要——别让 /reset 抹掉积累的长期记忆。
func (l *Layered) Reset(ctx context.Context, base string) error {
	today := l.now()
	_ = l.base.Reset(ctx, dayKey(base, today, "digest"))
	return l.base.Reset(ctx, dayKey(base, today, "raw"))
}

// Flush 同步刷新当天摘要——进程退出前调一次,把异步还没来得及做的补上,避免短会话丢摘要。
func (l *Layered) Flush(ctx context.Context, base string) {
	if l.summarize == nil {
		return
	}
	l.refreshDigest(base, l.now())
}

// pruneLocked 删掉早于保留期的天,返回保留下来的日期(升序)。调用方持锁。
func (l *Layered) pruneLocked(ctx context.Context, base, today string) []string {
	dates := l.loadIndex(ctx, base)
	if l.retentionDays <= 0 {
		return dates
	}
	cutoff := ""
	if t, err := time.Parse("2006-01-02", today); err == nil {
		cutoff = t.AddDate(0, 0, -l.retentionDays).Format("2006-01-02")
	}
	kept := make([]string, 0, len(dates))
	changed := false
	for _, d := range dates {
		if cutoff != "" && d < cutoff { // ISO 日期串比较即时间比较
			_ = l.base.Reset(ctx, dayKey(base, d, "raw"))
			_ = l.base.Reset(ctx, dayKey(base, d, "digest"))
			changed = true
			continue
		}
		kept = append(kept, d)
	}
	if changed {
		l.saveIndex(ctx, base, kept)
	}
	return kept
}

func (l *Layered) ensureIndexed(ctx context.Context, base, date string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	dates := l.loadIndex(ctx, base)
	for _, d := range dates {
		if d == date {
			return
		}
	}
	dates = append(dates, date)
	sort.Strings(dates)
	l.saveIndex(ctx, base, dates)
}

func (l *Layered) loadIndex(ctx context.Context, base string) []string {
	msgs, _ := l.base.Load(ctx, dayIndexKey(base))
	if len(msgs) == 0 {
		return nil
	}
	var dates []string
	_ = json.Unmarshal([]byte(msgs[len(msgs)-1].Content), &dates)
	return dates
}

func (l *Layered) saveIndex(ctx context.Context, base string, dates []string) {
	_ = l.base.Reset(ctx, dayIndexKey(base))
	b, _ := json.Marshal(dates)
	_ = l.base.Append(ctx, dayIndexKey(base), harness.Message{Role: "system", Content: string(b)})
}

// refreshDigest 重新把当天原始对话总结成一份摘要并覆盖写回。
func (l *Layered) refreshDigest(base, date string) {
	const method = "Layered.refreshDigest"
	ctx := context.Background()
	l.mu.Lock()
	defer l.mu.Unlock()
	raw, err := l.base.Load(ctx, dayKey(base, date, "raw"))
	if err != nil || len(raw) == 0 {
		return
	}
	digest, err := l.summarize(ctx, renderConversation(raw))
	if err != nil {
		log.Printf("method=%s base=%s date=%s err=%v", method, base, date, err)
		return
	}
	digest = truncateRunes(strings.TrimSpace(digest), l.summaryMaxChars)
	_ = l.base.Reset(ctx, dayKey(base, date, "digest"))
	_ = l.base.Append(ctx, dayKey(base, date, "digest"), harness.Message{Role: "system", Content: digest})
}

// renderConversation 把消息列表渲染成给总结模型看的纯文本。
func renderConversation(msgs []harness.Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case "user":
			sb.WriteString("用户: " + m.Content + "\n")
		case "assistant":
			if strings.TrimSpace(m.Content) != "" {
				sb.WriteString("助手: " + m.Content + "\n")
			}
		case "tool":
			sb.WriteString("[工具结果] " + truncateRunes(m.Content, 200) + "\n")
		}
	}
	return sb.String()
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i]
		}
		n++
	}
	return s
}
