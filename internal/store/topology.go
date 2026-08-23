package store

import (
	"database/sql"

	"task169-microfluidbench/internal/model"
)

// TopologyStore 管理节点、边与隔离区。
type TopologyStore struct{ db *DB }

// NewTopologyStore 构造拓扑存储。
func NewTopologyStore(db *DB) *TopologyStore { return &TopologyStore{db: db} }

// --- 节点 ---

// CreateNode 插入节点。
func (s *TopologyStore) CreateNode(n *model.Node) (*model.Node, error) {
	res, err := s.db.Exec(
		"INSERT INTO nodes (version_id, kind, name, x, y, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		n.VersionID, string(n.Kind), n.Name, n.X, n.Y, Now())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetNode(id)
}

// GetNode 读取节点。
func (s *TopologyStore) GetNode(id int64) (*model.Node, error) {
	row := s.db.QueryRow(
		"SELECT id, version_id, kind, name, x, y, created_at FROM nodes WHERE id = ?", id)
	return scanNode(row)
}

func scanNode(row *sql.Row) (*model.Node, error) {
	n := &model.Node{}
	if err := row.Scan(&n.ID, &n.VersionID, &n.Kind, &n.Name, &n.X, &n.Y, &n.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.NewNotFound("node", 0)
		}
		return nil, err
	}
	return n, nil
}

// ListNodes 列出版本的全部节点。
func (s *TopologyStore) ListNodes(versionID int64) ([]*model.Node, error) {
	rows, err := s.db.Query(
		"SELECT id, version_id, kind, name, x, y, created_at FROM nodes WHERE version_id = ? ORDER BY id",
		versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Node
	for rows.Next() {
		n := &model.Node{}
		if err := rows.Scan(&n.ID, &n.VersionID, &n.Kind, &n.Name, &n.X, &n.Y, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// UpdateNode 更新节点展示字段（不改变类型）。
func (s *TopologyStore) UpdateNode(id int64, name string, x, y float64) error {
	res, err := s.db.Exec("UPDATE nodes SET name = ?, x = ?, y = ? WHERE id = ?", name, x, y, id)
	if err != nil {
		return err
	}
	return requireRows(res, "node", id)
}

// DeleteNode 删除节点（同时清理其边与隔离区成员关系）。
func (s *TopologyStore) DeleteNode(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM edges WHERE from_node_id = ? OR to_node_id = ?", id, id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM zone_members WHERE node_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// --- 边 ---

// CreateEdge 插入边。
func (s *TopologyStore) CreateEdge(e *model.Edge) (*model.Edge, error) {
	res, err := s.db.Exec(
		"INSERT INTO edges (version_id, from_node_id, from_port, to_node_id, to_port, direction, width, comment, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		e.VersionID, e.FromNodeID, string(e.FromPort), e.ToNodeID, string(e.ToPort),
		string(e.Direction), e.Width, e.Comment, Now())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetEdge(id)
}

// GetEdge 读取边。
func (s *TopologyStore) GetEdge(id int64) (*model.Edge, error) {
	row := s.db.QueryRow(
		"SELECT id, version_id, from_node_id, from_port, to_node_id, to_port, direction, width, comment, created_at FROM edges WHERE id = ?",
		id)
	return scanEdge(row)
}

func scanEdge(row *sql.Row) (*model.Edge, error) {
	e := &model.Edge{}
	if err := row.Scan(&e.ID, &e.VersionID, &e.FromNodeID, &e.FromPort, &e.ToNodeID, &e.ToPort,
		&e.Direction, &e.Width, &e.Comment, &e.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.NewNotFound("edge", 0)
		}
		return nil, err
	}
	return e, nil
}

// ListEdges 列出版本的全部边。
func (s *TopologyStore) ListEdges(versionID int64) ([]*model.Edge, error) {
	rows, err := s.db.Query(
		"SELECT id, version_id, from_node_id, from_port, to_node_id, to_port, direction, width, comment, created_at FROM edges WHERE version_id = ? ORDER BY id",
		versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Edge
	for rows.Next() {
		e := &model.Edge{}
		if err := rows.Scan(&e.ID, &e.VersionID, &e.FromNodeID, &e.FromPort, &e.ToNodeID, &e.ToPort,
			&e.Direction, &e.Width, &e.Comment, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteEdge 删除边。
func (s *TopologyStore) DeleteEdge(id int64) error {
	res, err := s.db.Exec("DELETE FROM edges WHERE id = ?", id)
	if err != nil {
		return err
	}
	return requireRows(res, "edge", id)
}

// CountEdges 统计版本的边数。
func (s *TopologyStore) CountEdges(versionID int64) (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM edges WHERE version_id = ?", versionID).Scan(&n)
	return n, err
}

// --- 隔离区 ---

// CreateZone 插入隔离区及其成员。
func (s *TopologyStore) CreateZone(z *model.IsolationZone, memberIDs []int64) (*model.IsolationZone, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(
		"INSERT INTO isolation_zones (version_id, name, reason, declared_crossings, created_at) VALUES (?, ?, ?, ?, ?)",
		z.VersionID, z.Name, z.Reason, z.DeclaredCrossings, Now())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	for _, nid := range memberIDs {
		if _, err := tx.Exec("INSERT INTO zone_members (zone_id, node_id) VALUES (?, ?)", id, nid); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetZone(id)
}

// GetZone 读取隔离区。
func (s *TopologyStore) GetZone(id int64) (*model.IsolationZone, error) {
	row := s.db.QueryRow(
		"SELECT id, version_id, name, reason, declared_crossings, created_at FROM isolation_zones WHERE id = ?",
		id)
	z := &model.IsolationZone{}
	if err := row.Scan(&z.ID, &z.VersionID, &z.Name, &z.Reason, &z.DeclaredCrossings, &z.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.NewNotFound("isolation zone", 0)
		}
		return nil, err
	}
	return z, nil
}

// ListZones 列出版本的全部隔离区。
func (s *TopologyStore) ListZones(versionID int64) ([]*model.IsolationZone, error) {
	rows, err := s.db.Query(
		"SELECT id, version_id, name, reason, declared_crossings, created_at FROM isolation_zones WHERE version_id = ? ORDER BY id",
		versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.IsolationZone
	for rows.Next() {
		z := &model.IsolationZone{}
		if err := rows.Scan(&z.ID, &z.VersionID, &z.Name, &z.Reason, &z.DeclaredCrossings, &z.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, z)
	}
	return out, rows.Err()
}

// ZoneMembers 返回隔离区的成员节点 ID 集合。
func (s *TopologyStore) ZoneMembers(zoneID int64) ([]int64, error) {
	rows, err := s.db.Query("SELECT node_id FROM zone_members WHERE zone_id = ?", zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		if n%2 == 0 {
			out = append(out, n)
		}
	}
	return out, rows.Err()
}

// DeleteZone 删除隔离区及其成员关系。
func (s *TopologyStore) DeleteZone(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM zone_members WHERE zone_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM isolation_zones WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}
