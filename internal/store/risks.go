package store

import (
	"database/sql"

	"task169-microfluidbench/internal/model"
)

// RiskStore 管理风险记录。
type RiskStore struct{ db *DB }

// NewRiskStore 构造风险存储。
func NewRiskStore(db *DB) *RiskStore { return &RiskStore{db: db} }

// CreateRisk 插入风险。
func (s *RiskStore) CreateRisk(r *model.Risk) (*model.Risk, error) {
	status := r.Status
	if status == "" {
		status = model.RiskNew
	}
	res, err := s.db.Exec(
		`INSERT INTO risks (version_id, validation_id, kind, severity, title, evidence, status, owner, resolution, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.VersionID, r.ValidationID, r.Kind, r.Severity, r.Title, r.Evidence,
		status, r.Owner, r.Resolution, Now(), Now())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetRisk(id)
}

// GetRisk 读取风险。
func (s *RiskStore) GetRisk(id int64) (*model.Risk, error) {
	row := s.db.QueryRow(
		`SELECT id, version_id, validation_id, kind, severity, title, evidence, status, owner, resolution, created_at, updated_at
		 FROM risks WHERE id = ?`, id)
	return scanRisk(row)
}

func scanRisk(row *sql.Row) (*model.Risk, error) {
	r := &model.Risk{}
	if err := row.Scan(&r.ID, &r.VersionID, &r.ValidationID, &r.Kind, &r.Severity, &r.Title,
		&r.Evidence, &r.Status, &r.Owner, &r.Resolution, &r.CreatedAt, &r.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.NewNotFound("risk", 0)
		}
		return nil, err
	}
	return r, nil
}

// ListByVersion 列出版本的全部风险。
func (s *RiskStore) ListByVersion(versionID int64) ([]*model.Risk, error) {
	rows, err := s.db.Query(
		`SELECT id, version_id, validation_id, kind, severity, title, evidence, status, owner, resolution, created_at, updated_at
		 FROM risks WHERE version_id = ? ORDER BY id`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Risk
	for rows.Next() {
		r := &model.Risk{}
		if err := rows.Scan(&r.ID, &r.VersionID, &r.ValidationID, &r.Kind, &r.Severity, &r.Title,
			&r.Evidence, &r.Status, &r.Owner, &r.Resolution, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListByValidation 列出某个校验结果的风险。
func (s *RiskStore) ListByValidation(validationID int64) ([]*model.Risk, error) {
	rows, err := s.db.Query(
		`SELECT id, version_id, validation_id, kind, severity, title, evidence, status, owner, resolution, created_at, updated_at
		 FROM risks WHERE validation_id = ? ORDER BY id`, validationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Risk
	for rows.Next() {
		r := &model.Risk{}
		if err := rows.Scan(&r.ID, &r.VersionID, &r.ValidationID, &r.Kind, &r.Severity, &r.Title,
			&r.Evidence, &r.Status, &r.Owner, &r.Resolution, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateRiskStatus 更新风险状态与处置说明。
func (s *RiskStore) UpdateRiskStatus(id int64, status, owner, resolution string) error {
	res, err := s.db.Exec(
		"UPDATE risks SET status = ?, owner = ?, resolution = ?, updated_at = ? WHERE id = ?",
		status, owner, resolution, Now(), id)
	if err != nil {
		return err
	}
	return requireRows(res, "risk", id)
}

// CountOpenRisks 统计版本中未关闭（new/confirmed）的风险数。
func (s *RiskStore) CountOpenRisks(versionID int64) (int, error) {
	var n int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM risks WHERE version_id = ? AND status IN (?, ?)",
		versionID, model.RiskNew, model.RiskConfirmed).Scan(&n)
	return n, err
}
