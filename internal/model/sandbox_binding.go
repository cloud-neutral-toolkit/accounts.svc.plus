package model

import (
	"time"
)

// SandboxBinding represents the binding between the Sandbox mode and a specific agent.
type SandboxBinding struct {
	ID        uint      `gorm:"primaryKey"`
	AgentID   string    `gorm:"column:agent_id;type:text;not null;uniqueIndex"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

// TableName overrides the default table name used by GORM.
func (SandboxBinding) TableName() string { return "sandbox_bindings" }
