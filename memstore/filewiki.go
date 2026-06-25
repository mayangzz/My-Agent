package memstore

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mayangzz/My-Agent/harness"
)

// FileWiki 把每个 session 的对话存成本地一个 .jsonl 文件(逐条消息一行)。
// 无需任何 DB / 容器:落盘持久、能直接打开看;JSONL 忠实保留工具调用,重启可完整还原。
type FileWiki struct {
	dir string
	mu  sync.Mutex
}

// NewFileWiki 在 dir 下存记忆文件(空则用 "memory")。
func NewFileWiki(dir string) (*FileWiki, error) {
	const method = "memstore.NewFileWiki"
	if dir == "" {
		dir = "memory"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("method=%s mkdir %s: %w", method, dir, err)
	}
	return &FileWiki{dir: dir}, nil
}

// path 把 session 名映射成文件路径。session 里的 "/" 当成子目录分隔
// (于是分层记忆的 <base>/<date>/raw 落成按天分的目录),每段都做安全化、挡路径穿越。
func (f *FileWiki) path(session string) string {
	parts := strings.Split(session, "/")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Map(func(r rune) rune {
			switch r {
			case '\\', ':', '*', '?', '"', '<', '>', '|':
				return '_'
			}
			return r
		}, p)
		if p == "" || p == "." || p == ".." {
			p = "_"
		}
		clean = append(clean, p)
	}
	return filepath.Join(f.dir, filepath.Join(clean...)+".jsonl")
}

func (f *FileWiki) Append(ctx context.Context, session string, m harness.Message) error {
	const method = "FileWiki.Append"
	f.mu.Lock()
	defer f.mu.Unlock()
	line, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("method=%s marshal: %w", method, err)
	}
	p := f.path(session)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { // 分层 key 会落到子目录
		return fmt.Errorf("method=%s mkdir: %w", method, err)
	}
	file, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("method=%s open: %w", method, err)
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("method=%s write: %w", method, err)
	}
	return nil
}

func (f *FileWiki) Load(ctx context.Context, session string) ([]harness.Message, error) {
	const method = "FileWiki.Load"
	f.mu.Lock()
	defer f.mu.Unlock()
	file, err := os.Open(f.path(session))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("method=%s open: %w", method, err)
	}
	defer file.Close()

	var out []harness.Message
	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // 单条消息可能很长
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var m harness.Message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			return nil, fmt.Errorf("method=%s unmarshal: %w", method, err)
		}
		out = append(out, m)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("method=%s scan: %w", method, err)
	}
	return out, nil
}

func (f *FileWiki) Reset(ctx context.Context, session string) error {
	const method = "FileWiki.Reset"
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := os.Remove(f.path(session)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("method=%s remove: %w", method, err)
	}
	return nil
}
