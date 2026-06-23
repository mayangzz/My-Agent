package harness

import "context"

// Memory 是会话记忆的抽象:按 session 存取消息历史。
// harness 只认这个接口,具体存哪(内存/Postgres/Redis)由 memstore 包实现、启动时注入。
type Memory interface {
	Append(ctx context.Context, session string, m Message) error // 追加一条消息
	Load(ctx context.Context, session string) ([]Message, error) // 取该 session 的全部历史(不含 system)
	Reset(ctx context.Context, session string) error             // 清空该 session
}
