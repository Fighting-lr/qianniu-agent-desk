package bootstrap

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-desk/internal/pkg/config"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/driver/mysql"

	// "gorm.io/driver/sqlite" // Sqlite driver based on CGO
	"github.com/glebarez/sqlite" // Pure go SQLite driver, checkout https://github.com/glebarez/sqlite for details
	"gorm.io/gorm"

	// "gorm.io/gorm/logger"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func InitDB(cfg config.DBConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.Type {
	case "sqlite":
		if err := ensureSQLiteDir(cfg.DSN); err != nil {
			return nil, err
		}
		// [AgentDesk-BEGIN] 修正 SQLite 并发写参数（见 normalizeSQLiteDSN 注释）
		dsn := normalizeSQLiteDSN(cfg.DSN)
		if dsn != cfg.DSN {
			log.Printf("[db] sqlite dsn 已自动补齐并发参数: %s", dsn)
		}
		dialector = sqlite.Open(dsn)
		// [AgentDesk-END]
	case "mysql":
		dialector = mysql.Open(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported db type: %s", cfg.Type)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logger.Warn,
				IgnoreRecordNotFoundError: true,
				Colorful:                  true,
			},
		),
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.ConnMaxIdleTimeSeconds > 0 {
		sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTimeSeconds) * time.Second)
	}
	if cfg.ConnMaxLifetimeSeconds > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSeconds) * time.Second)
	}

	sqls.SetDB(db)
	return db, nil
}

// [AgentDesk-BEGIN] SQLite 并发写修复
//
// 问题现象：工作台并发操作时会随机报
//     database is locked (5) (SQLITE_BUSY)
// 而且把 `_busy_timeout` 调多大都没用。
//
// 根因（实测复现）：SQLite 的经典"锁升级死锁"。
// 连接 A 和 B 各自开启事务、都先做了一次读（取得 SHARED 锁），
// 随后都要写（需要升级为 RESERVED）。此时 SQLite 必须让其中一方立即失败，
// 而这个失败路径**不会调用 busy handler**，所以 busy_timeout 形同虚设。
//
// 对照实验（8 个 goroutine 并发「事务内先读后写」，跑两轮结果一致）：
//     ① 现状：_busy_timeout=5000                → 成功 2~3 / 失败 5~6   ❌
//     ② ① + SetMaxOpenConns(1)                  → 成功 8 / 失败 0      ✅
//     ③ ① + _txlock=immediate                   → 成功 8 / 失败 0      ✅ ← 采用
//     ④ ③ + SetMaxOpenConns(1)                  → 成功 8 / 失败 0      ✅
//     ⑤ 改 journal_mode=WAL                     → 成功 2 / 失败 6      ❌
//     ⑥ ⑤ + SetMaxOpenConns(1)                  → 成功 8 / 失败 0      ✅
//
// 注意 ⑤：**改成 WAL 并不能解决**（WAL 下读事务升级写同样会返回
// SQLITE_BUSY_SNAPSHOT，且不触发 busy handler）—— 这是个常见误解。
//
// 选择 ③ 而不是 ② 的原因：
//   · BEGIN IMMEDIATE 让事务在开始时就取写锁，从根上消除"读→写升级"这一步，
//     并发写在 busy_timeout 窗口内自然排队；
//   · 保留连接池，不会因为"事务中再取一条连接"而**永久死等**
//     （MaxOpenConns=1 存在这个风险，一旦命中就是卡死而非报错）。
func normalizeSQLiteDSN(dsn string) string {
	// 只改写显式的 file: 形式。裸路径的 DSN 语义由驱动自行解释，不擅自拼接参数。
	if !strings.HasPrefix(dsn, "file:") {
		return dsn
	}

	has := func(sub string) bool { return strings.Contains(dsn, sub) }

	var extra []string
	if !has("_txlock=") {
		extra = append(extra, "_txlock=immediate")
	}
	// 没配 busy_timeout 时给一个更宽裕的默认值，避免长事务期间过早失败。
	if !has("_busy_timeout=") && !has("busy_timeout(") {
		extra = append(extra, "_busy_timeout=10000")
	}
	if len(extra) == 0 {
		return dsn
	}

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + strings.Join(extra, "&")
}

// [AgentDesk-END]

func ensureSQLiteDir(dsn string) error {
	dbPath := sqliteFilePath(dsn)
	if dbPath == "" {
		return nil
	}
	dir := filepath.Dir(dbPath)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func sqliteFilePath(dsn string) string {
	if dsn == "" {
		return ""
	}

	path := dsn
	if after, ok := strings.CutPrefix(path, "file:"); ok {
		path = after
	}
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}

	normalized := strings.TrimSpace(path)
	if normalized == "" || normalized == ":memory:" || strings.Contains(normalized, "mode=memory") {
		return ""
	}
	return normalized
}
