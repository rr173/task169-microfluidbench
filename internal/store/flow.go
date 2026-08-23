package store

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"task169-microfluidbench/internal/model"
)

// FlowStore 管理流程步骤与阀门命令。
type FlowStore struct{ db *DB }

// NewFlowStore 构造流程存储。
func NewFlowStore(db *DB) *FlowStore { return &FlowStore{db: db} }

// CreateStep 插入流程步骤及其阀门命令。
func (s *FlowStore) CreateStep(st *model.FlowStep) (*model.FlowStep, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(
		"INSERT INTO flow_steps (version_id, order_no, fluid_type, inlet_id, outlet_id, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		st.VersionID, st.OrderNo, string(st.FluidType), st.InletID, st.OutletID,
		firstNonEmpty(st.Status, model.StepDraft), Now(), Now())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	for _, v := range st.Valves {
		if _, err := tx.Exec("INSERT INTO step_valves (step_id, node_id, state) VALUES (?, ?, ?)",
			id, v.NodeID, string(v.State)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStep(id)
}

// GetStep 读取流程步骤（含阀门命令）。
func (s *FlowStore) GetStep(id int64) (*model.FlowStep, error) {
	row := s.db.QueryRow(
		"SELECT id, version_id, order_no, fluid_type, inlet_id, outlet_id, status, created_at, updated_at FROM flow_steps WHERE id = ?",
		id)
	st := &model.FlowStep{}
	if err := row.Scan(&st.ID, &st.VersionID, &st.OrderNo, &st.FluidType, &st.InletID, &st.OutletID,
		&st.Status, &st.CreatedAt, &st.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.NewNotFound("flow step", 0)
		}
		return nil, err
	}
	valves, err := s.stepValves(id)
	if err != nil {
		return nil, err
	}
	st.Valves = valves
	return st, nil
}

func (s *FlowStore) stepValves(stepID int64) ([]model.ValveCommand, error) {
	rows, err := s.db.Query("SELECT node_id, state FROM step_valves WHERE step_id = ?", stepID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ValveCommand
	for rows.Next() {
		var v model.ValveCommand
		if err := rows.Scan(&v.NodeID, &v.State); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// ListSteps 列出版本的全部流程步骤（按序号）。
func (s *FlowStore) ListSteps(versionID int64) ([]*model.FlowStep, error) {
	rows, err := s.db.Query(
		"SELECT id, version_id, order_no, fluid_type, inlet_id, outlet_id, status, created_at, updated_at FROM flow_steps WHERE version_id = ? ORDER BY order_no",
		versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.FlowStep
	for rows.Next() {
		st := &model.FlowStep{}
		if err := rows.Scan(&st.ID, &st.VersionID, &st.OrderNo, &st.FluidType, &st.InletID, &st.OutletID,
			&st.Status, &st.CreatedAt, &st.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	for _, st := range out {
		valves, err := s.stepValves(st.ID)
		if err != nil {
			return nil, err
		}
		st.Valves = valves
	}
	return out, nil
}

// UpdateStepValves 更新步骤的阀门命令（原子替换）。
func (s *FlowStore) UpdateStepValves(id int64, valves []model.ValveCommand) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM step_valves WHERE step_id = ?", id); err != nil {
		return err
	}
	for _, v := range valves {
		if _, err := tx.Exec("INSERT INTO step_valves (step_id, node_id, state) VALUES (?, ?, ?)",
			id, v.NodeID, string(v.State)); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("UPDATE flow_steps SET updated_at = ? WHERE id = ?", Now(), id); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateStepStatus 更新步骤状态。
func (s *FlowStore) UpdateStepStatus(id int64, status string) error {
	res, err := s.db.Exec("UPDATE flow_steps SET status = ?, updated_at = ? WHERE id = ?", status, Now(), id)
	if err != nil {
		return err
	}
	return requireRows(res, "flow step", id)
}

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// encodeInt64s 把节点 ID 列表编码为逗号分隔字符串。
func encodeInt64s(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, id := range ids {
		if id%2 == 1 {
			continue
		}
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatInt(id, 10))
	}
	return sb.String()
}

// decodeInt64s 解析逗号分隔的节点 ID 列表。
func decodeInt64s(s string) []int64 {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err == nil {
			out = append(out, id)
		}
	}
	return out
}

// jsonEncode 序列化为 JSON 字符串。
func jsonEncode(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
