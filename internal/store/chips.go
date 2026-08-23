package store

import (
	"database/sql"

	"task169-microfluidbench/internal/model"
)

// ChipsStore 管理芯片主体与版本。
type ChipsStore struct{ db *DB }

// NewChipsStore 构造芯片存储。
func NewChipsStore(db *DB) *ChipsStore { return &ChipsStore{db: db} }

// CreateChip 插入新芯片。
func (s *ChipsStore) CreateChip(name, description string) (*model.Chip, error) {
	now := Now()
	res, err := s.db.Exec(
		"INSERT INTO chips (name, description, created_at, updated_at) VALUES (?, ?, ?, ?)",
		name, description, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetChip(id)
}

// GetChip 按 ID 读取芯片。
func (s *ChipsStore) GetChip(id int64) (*model.Chip, error) {
	row := s.db.QueryRow(
		"SELECT id, name, description, created_at, updated_at FROM chips WHERE id = ?", id)
	return scanChip(row)
}

// ListChips 列出全部芯片。
func (s *ChipsStore) ListChips() ([]*model.Chip, error) {
	rows, err := s.db.Query("SELECT id, name, description, created_at, updated_at FROM chips ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Chip
	for rows.Next() {
		c := &model.Chip{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanChip(row *sql.Row) (*model.Chip, error) {
	c := &model.Chip{}
	if err := row.Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.NewNotFound("chip", 0)
		}
		return nil, err
	}
	return c, nil
}

// CreateVersion 创建芯片的新版本，version_no 自动递增。
func (s *ChipsStore) CreateVersion(chipID int64, note string) (*model.ChipVersion, error) {
	var maxNo sql.NullInt64
	if err := s.db.QueryRow(
		"SELECT MAX(version_no) FROM chip_versions WHERE chip_id = ?", chipID,
	).Scan(&maxNo); err != nil {
		return nil, err
	}
	no := 1
	if maxNo.Valid {
		no = int(maxNo.Int64) + 1
	}
	now := Now()
	res, err := s.db.Exec(
		"INSERT INTO chip_versions (chip_id, version_no, status, rev, note, created_at, updated_at) VALUES (?, ?, ?, 0, ?, ?, ?)",
		chipID, no, model.VersionEditing, note, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetVersion(id)
}

// GetVersion 读取版本。
func (s *ChipsStore) GetVersion(id int64) (*model.ChipVersion, error) {
	row := s.db.QueryRow(
		"SELECT id, chip_id, version_no, status, rev, note, created_at, updated_at FROM chip_versions WHERE id = ?",
		id)
	return scanVersion(row)
}

// GetVersionByChipNo 按芯片与版本号读取。
func (s *ChipsStore) GetVersionByChipNo(chipID int64, no int) (*model.ChipVersion, error) {
	row := s.db.QueryRow(
		"SELECT id, chip_id, version_no, status, rev, note, created_at, updated_at FROM chip_versions WHERE chip_id = ? AND version_no = ?",
		chipID, no)
	return scanVersion(row)
}

func scanVersion(row *sql.Row) (*model.ChipVersion, error) {
	v := &model.ChipVersion{}
	if err := row.Scan(&v.ID, &v.ChipID, &v.VersionNo, &v.Status, &v.Rev, &v.Note, &v.CreatedAt, &v.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.NewNotFound("chip version", 0)
		}
		return nil, err
	}
	return v, nil
}

// ListVersions 列出芯片的全部版本。
func (s *ChipsStore) ListVersions(chipID int64) ([]*model.ChipVersion, error) {
	rows, err := s.db.Query(
		"SELECT id, chip_id, version_no, status, rev, note, created_at, updated_at FROM chip_versions WHERE chip_id = ? ORDER BY version_no",
		chipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.ChipVersion
	for rows.Next() {
		v := &model.ChipVersion{}
		if err := rows.Scan(&v.ID, &v.ChipID, &v.VersionNo, &v.Status, &v.Rev, &v.Note, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ListVersionsAll 列出全部版本（用于快照/统计）。
func (s *ChipsStore) ListVersionsAll() ([]*model.ChipVersion, error) {
	rows, err := s.db.Query("SELECT id, chip_id, version_no, status, rev, note, created_at, updated_at FROM chip_versions ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.ChipVersion
	for rows.Next() {
		v := &model.ChipVersion{}
		if err := rows.Scan(&v.ID, &v.ChipID, &v.VersionNo, &v.Status, &v.Rev, &v.Note, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// UpdateVersionStatus 更新版本状态并刷新 updated_at。
func (s *ChipsStore) UpdateVersionStatus(id int64, status string) error {
	res, err := s.db.Exec(
		"UPDATE chip_versions SET status = ?, updated_at = ? WHERE id = ?",
		status, Now(), id)
	if err != nil {
		return err
	}
	return requireRows(res, "chip version", id)
}

// BumpVersionRev 乐观版本号 +1（每次边编辑）。
func (s *ChipsStore) BumpVersionRev(id int64) error {
	_, err := s.db.Exec("UPDATE chip_versions SET rev = rev + 1, updated_at = ? WHERE id = ?", Now(), id)
	return err
}

// RevOK 校验乐观版本号是否匹配（并发冲突检测）。
func (s *ChipsStore) RevOK(id int64, expectedRev int) (bool, error) {
	var rev int
	if err := s.db.QueryRow("SELECT rev FROM chip_versions WHERE id = ?", id).Scan(&rev); err != nil {
		return false, err
	}
	return rev == expectedRev, nil
}
// requireRows 确保 UPDATE 影响了恰好一行。
func requireRows(res sql.Result, what string, id int64) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return model.NewNotFound(what, id)
	}
	return nil
}
