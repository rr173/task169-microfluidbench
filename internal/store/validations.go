package store

import (
	"database/sql"

	"task169-microfluidbench/internal/model"
)

// ValidationStore 管理校验结果。
type ValidationStore struct{ db *DB }

// NewValidationStore 构造校验存储。
func NewValidationStore(db *DB) *ValidationStore { return &ValidationStore{db: db} }

// SaveValidation 保存校验结果。
func (s *ValidationStore) SaveValidation(v *model.ValidationResult) (*model.ValidationResult, error) {
	passed := 0
	if v.Passed {
		passed = 1
	}
	reachable := 0
	if v.Reachable {
		reachable = 1
	}
	res, err := s.db.Exec(
		`INSERT INTO validation_results
		 (step_id, version_id, passed, reachable, reachable_path, blocked_edges,
		  dead_volumes, cross_contam_edges, residual_wells, isolation_breaks, message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.StepID, v.VersionID, passed, reachable,
		encodeInt64s(v.ReachablePath), encodeInt64s(v.BlockedEdges),
		encodeInt64s(v.DeadVolumes), encodeInt64s(v.CrossContamEdges),
		encodeInt64s(v.ResidualWells), encodeInt64s(v.IsolationBreaks),
		v.Message, Now())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetValidation(id)
}

// GetValidation 读取校验结果。
func (s *ValidationStore) GetValidation(id int64) (*model.ValidationResult, error) {
	row := s.db.QueryRow(
		`SELECT id, step_id, version_id, passed, reachable, reachable_path, blocked_edges,
		        dead_volumes, cross_contam_edges, residual_wells, isolation_breaks, message, created_at
		 FROM validation_results WHERE id = ?`, id)
	return scanValidation(row)
}

func scanValidation(row *sql.Row) (*model.ValidationResult, error) {
	v := &model.ValidationResult{}
	var passed, reachable int
	var pathStr, blockedStr, deadStr, contamStr, residStr, isoStr string
	if err := row.Scan(&v.ID, &v.StepID, &v.VersionID, &passed, &reachable,
		&pathStr, &blockedStr, &deadStr, &contamStr, &residStr, &isoStr,
		&v.Message, &v.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.NewNotFound("validation", 0)
		}
		return nil, err
	}
	v.Passed = passed == 1
	v.Reachable = reachable == 1
	v.ReachablePath = decodeInt64s(pathStr)
	v.BlockedEdges = decodeInt64s(blockedStr)
	v.DeadVolumes = decodeInt64s(deadStr)
	v.CrossContamEdges = decodeInt64s(contamStr)
	v.ResidualWells = decodeInt64s(residStr)
	v.IsolationBreaks = decodeInt64s(isoStr)
	return v, nil
}

// ListByVersion 列出版本的全部校验结果（按时间倒序）。
func (s *ValidationStore) ListByVersion(versionID int64) ([]*model.ValidationResult, error) {
	rows, err := s.db.Query(
		`SELECT id, step_id, version_id, passed, reachable, reachable_path, blocked_edges,
		        dead_volumes, cross_contam_edges, residual_wells, isolation_breaks, message, created_at
		 FROM validation_results WHERE version_id = ? ORDER BY id DESC`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.ValidationResult
	for rows.Next() {
		v := &model.ValidationResult{}
		var passed, reachable int
		var pathStr, blockedStr, deadStr, contamStr, residStr, isoStr string
		if err := rows.Scan(&v.ID, &v.StepID, &v.VersionID, &passed, &reachable,
			&pathStr, &blockedStr, &deadStr, &contamStr, &residStr, &isoStr,
			&v.Message, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.Passed = passed == 1
		v.Reachable = reachable == 1
		v.ReachablePath = decodeInt64s(pathStr)
		v.BlockedEdges = decodeInt64s(blockedStr)
		v.DeadVolumes = decodeInt64s(deadStr)
		v.CrossContamEdges = decodeInt64s(contamStr)
		v.ResidualWells = decodeInt64s(residStr)
		v.IsolationBreaks = decodeInt64s(isoStr)
		out = append(out, v)
	}
	return out, rows.Err()
}

// CountValidationIssues 统计版本中未通过校验的步数。
func (s *ValidationStore) CountValidationIssues(versionID int64) (int, error) {
	var n int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM validation_results WHERE version_id = ? AND passed = 0", versionID).Scan(&n)
	return n, err
}
