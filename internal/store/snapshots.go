package store

import (
	"database/sql"

	"task169-microfluidbench/internal/model"
)

// SnapshotStore 管理审查快照。
type SnapshotStore struct{ db *DB }

// NewSnapshotStore 构造快照存储。
func NewSnapshotStore(db *DB) *SnapshotStore { return &SnapshotStore{db: db} }

// CreateSnapshot 创建构建中快照。
func (s *SnapshotStore) CreateSnapshot(versionID int64, frozenBy string) (*model.Snapshot, error) {
	res, err := s.db.Exec(
		`INSERT INTO snapshots (version_id, status, topology_hash, valve_state_table, node_count, edge_count, frozen_by, created_at)
		 VALUES (?, ?, '', '', 0, 0, ?, ?)`,
		versionID, model.SnapshotBuilding, frozenBy, Now())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetSnapshot(id)
}

// GetSnapshot 读取快照。
func (s *SnapshotStore) GetSnapshot(id int64) (*model.Snapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, version_id, status, topology_hash, valve_state_table, node_count, edge_count, frozen_by, created_at, frozen_at
		 FROM snapshots WHERE id = ?`, id)
	return scanSnapshot(row)
}

func scanSnapshot(row *sql.Row) (*model.Snapshot, error) {
	s := &model.Snapshot{}
	var frozenAt sql.NullString
	if err := row.Scan(&s.ID, &s.VersionID, &s.Status, &s.TopologyHash, &s.ValveStateTable,
		&s.NodeCount, &s.EdgeCount, &s.FrozenBy, &s.CreatedAt, &frozenAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.NewNotFound("snapshot", 0)
		}
		return nil, err
	}
	if frozenAt.Valid {
		if t, err := model.ParseTime(frozenAt.String); err == nil {
			s.FrozenAt = &t
		}
	}
	return s, nil
}

// FreezeSnapshot 把构建中快照冻结为已冻结。
// valveTable 是已生成的阀门状态表（含关闭状态），冻结时必须原样保留，重启读取才能还原。
func (s *SnapshotStore) FreezeSnapshot(id int64, hash, valveTable string, nodeCount, edgeCount int) error {
	res, err := s.db.Exec(
		`UPDATE snapshots SET status = ?, topology_hash = ?, valve_state_table = ?, node_count = ?, edge_count = ?, frozen_at = ?
		 WHERE id = ?`,
		model.SnapshotFrozen, hash, valveTable, nodeCount, edgeCount, Now(), id)
	if err != nil {
		return err
	}
	return requireRows(res, "snapshot", id)
}

// ListByVersion 列出版本的全部快照。
func (s *SnapshotStore) ListByVersion(versionID int64) ([]*model.Snapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, version_id, status, topology_hash, valve_state_table, node_count, edge_count, frozen_by, created_at, frozen_at
		 FROM snapshots WHERE version_id = ? ORDER BY id`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Snapshot
	for rows.Next() {
		snap := &model.Snapshot{}
		var frozenAt sql.NullString
		if err := rows.Scan(&snap.ID, &snap.VersionID, &snap.Status, &snap.TopologyHash,
			&snap.ValveStateTable, &snap.NodeCount, &snap.EdgeCount, &snap.FrozenBy,
			&snap.CreatedAt, &frozenAt); err != nil {
			return nil, err
		}
		if frozenAt.Valid {
			if t, err := model.ParseTime(frozenAt.String); err == nil {
				snap.FrozenAt = &t
			}
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}
