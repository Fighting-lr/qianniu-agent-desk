package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteFilePath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "plain relative path",
			dsn:  "./data/app.db",
			want: "./data/app.db",
		},
		{
			name: "file uri with query",
			dsn:  "file:./data/app.db?_busy_timeout=5000",
			want: "./data/app.db",
		},
		{
			name: "memory dsn",
			dsn:  "file::memory:?cache=shared",
			want: "",
		},
		{
			name: "memory alias",
			dsn:  ":memory:",
			want: "",
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sqliteFilePath(tt.dsn); got != tt.want {
				t.Fatalf("sqliteFilePath(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
		})
	}
}

func TestEnsureSQLiteDir(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	dbPath := filepath.Join(baseDir, "nested", "app.db")
	dsn := "file:" + dbPath + "?_busy_timeout=5000"

	if err := ensureSQLiteDir(dsn); err != nil {
		t.Fatalf("ensureSQLiteDir() error = %v", err)
	}

	if info, err := os.Stat(filepath.Dir(dbPath)); err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	} else if !info.IsDir() {
		t.Fatalf("expected %q to be a directory", filepath.Dir(dbPath))
	}
}

// TestNormalizeSQLiteDSN 守护 SQLite 并发写的修复。
//
// 背景：不加 `_txlock=immediate` 时，并发「事务内先读后写」会随机抛
// SQLITE_BUSY（锁升级死锁，busy handler 不生效）。
// 若这里被改回原样，工作台会重新出现 database is locked。
func TestNormalizeSQLiteDSN(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "补齐 txlock，保留已有 busy_timeout",
			dsn:  "file:./data/app.db?_busy_timeout=5000",
			want: "file:./data/app.db?_busy_timeout=5000&_txlock=immediate",
		},
		{
			name: "无查询参数时用 ?",
			dsn:  "file:./data/app.db",
			want: "file:./data/app.db?_txlock=immediate&_busy_timeout=10000",
		},
		{
			name: "已有 _txlock 则不重复添加",
			dsn:  "file:./data/app.db?_txlock=immediate",
			want: "file:./data/app.db?_txlock=immediate&_busy_timeout=10000",
		},
		{
			name: "已用 _pragma 设置 busy_timeout 则不再追加",
			dsn:  "file:./data/app.db?_pragma=busy_timeout(5000)&_txlock=immediate",
			want: "file:./data/app.db?_pragma=busy_timeout(5000)&_txlock=immediate",
		},
		{
			name: "裸路径不改写",
			dsn:  "./data/app.db",
			want: "./data/app.db",
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeSQLiteDSN(tt.dsn); got != tt.want {
				t.Fatalf("normalizeSQLiteDSN(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
		})
	}
}
