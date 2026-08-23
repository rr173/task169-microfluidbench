// Package store 提供 SQLite 持久化：建表迁移、CRUD 与重启恢复。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB 封装 SQLite 连接。
type DB struct {
	*sql.DB
}

// Open 打开（必要时创建）SQLite 数据库并执行迁移。
// path 为 ":memory:" 或文件路径；文件模式会自动创建父目录。
func Open(path string) (*DB, error) {
	if path == "" {
		path = ":memory:"
	}
	if path != ":memory:" {
		if dir := filepath.Dir(path); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("创建数据库目录失败: %w", err)
			}
		}
	}
	dsn := path
	if path != ":memory:" {
		dsn = fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	}
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(1) // SQLite 单写者，避免锁竞争
	db := &DB{sqlDB}
	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	return db, nil
}

// migrate 执行幂等建表与版本迁移。
func (db *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER NOT NULL,
			applied_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS chips (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS chip_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chip_id INTEGER NOT NULL,
			version_no INTEGER NOT NULL,
			status TEXT NOT NULL,
			rev INTEGER NOT NULL DEFAULT 0,
			note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE (chip_id, version_no)
		)`,
		`CREATE TABLE IF NOT EXISTS nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version_id INTEGER NOT NULL,
			kind TEXT NOT NULL,
			name TEXT NOT NULL,
			x REAL NOT NULL DEFAULT 0,
			y REAL NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS edges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version_id INTEGER NOT NULL,
			from_node_id INTEGER NOT NULL,
			from_port TEXT NOT NULL,
			to_node_id INTEGER NOT NULL,
			to_port TEXT NOT NULL,
			direction TEXT NOT NULL,
			width REAL NOT NULL DEFAULT 10,
			comment TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_version ON edges(version_id)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_node_id)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_node_id)`,
		`CREATE TABLE IF NOT EXISTS isolation_zones (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			reason TEXT NOT NULL DEFAULT '',
			declared_crossings TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS zone_members (
			zone_id INTEGER NOT NULL,
			node_id INTEGER NOT NULL,
			PRIMARY KEY (zone_id, node_id)
		)`,
		`CREATE TABLE IF NOT EXISTS flow_steps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version_id INTEGER NOT NULL,
			order_no INTEGER NOT NULL,
			fluid_type TEXT NOT NULL,
			inlet_id INTEGER NOT NULL,
			outlet_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS step_valves (
			step_id INTEGER NOT NULL,
			node_id INTEGER NOT NULL,
			state TEXT NOT NULL,
			PRIMARY KEY (step_id, node_id)
		)`,
		`CREATE TABLE IF NOT EXISTS validation_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			step_id INTEGER NOT NULL,
			version_id INTEGER NOT NULL,
			passed INTEGER NOT NULL DEFAULT 0,
			reachable INTEGER NOT NULL DEFAULT 0,
			reachable_path TEXT NOT NULL DEFAULT '',
			blocked_edges TEXT NOT NULL DEFAULT '',
			dead_volumes TEXT NOT NULL DEFAULT '',
			cross_contam_edges TEXT NOT NULL DEFAULT '',
			residual_wells TEXT NOT NULL DEFAULT '',
			isolation_breaks TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS risks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version_id INTEGER NOT NULL,
			validation_id INTEGER NOT NULL DEFAULT 0,
			kind TEXT NOT NULL,
			severity TEXT NOT NULL DEFAULT 'medium',
			title TEXT NOT NULL,
			evidence TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'new',
			owner TEXT NOT NULL DEFAULT '',
			resolution TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'building',
			topology_hash TEXT NOT NULL DEFAULT '',
			valve_state_table TEXT NOT NULL DEFAULT '',
			node_count INTEGER NOT NULL DEFAULT 0,
			edge_count INTEGER NOT NULL DEFAULT 0,
			frozen_by TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			frozen_at TEXT
		)`,
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("执行迁移语句失败: %w\n语句: %s", err, s)
		}
	}
	if _, err := tx.Exec(
		"INSERT INTO schema_version (version, applied_at) VALUES (?, ?)",
		1, time.Now().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("记录 schema 版本失败: %w", err)
	}
	return tx.Commit()
}

// Close 关闭数据库连接。
func (db *DB) Close() error { return db.DB.Close() }

// Now 返回统一的 UTC 时间戳字符串，供持久化使用。
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

// ParseTime 解析持久化的时间戳。
func ParseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339, s) }
