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

// path 把 session 名映射成一个安全的文件名(挡掉路径分隔符等,保留中文)。
func (f *FileWiki) path(session string) string {
	safe := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, session)
	return filepath.Join(f.dir, safe+".jsonl")
}

func (f *FileWiki) Append(ctx context.Context, session string, m harness.Message) error {
	const method = "FileWiki.Append"
	f.mu.Lock()
	defer f.mu.Unlock()
	line, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("method=%s marshal: %w", method, err)
	}
	file, err := os.OpenFile(f.path(session), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
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
