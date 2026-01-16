package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AdminSetting represents a single permission toggle in the admin matrix.
type AdminSetting struct {
	UUID       string    `gorm:"column:uuid;type:uuid;primaryKey"`
	ModuleKey  string    `gorm:"column:module_key;type:text;not null;uniqueIndex:idx_admin_settings_module_role"`
	Role       string    `gorm:"column:role;type:text;not null;uniqueIndex:idx_admin_settings_module_role"`
	Enabled    bool      `gorm:"column:enabled;not null"`
	Version    uint64    `gorm:"column:version;type:bigint;not null;index"`
	OriginNode string    `gorm:"column:origin_node;type:text;not null;default:local"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

// TableName overrides the default table name used by GORM.
func (AdminSetting) TableName() string { return "admin_settings" }

// BeforeCreate ensures the primary key and defaults align with the database schema.
func (setting *AdminSetting) BeforeCreate(tx *gorm.DB) error {
	if strings.TrimSpace(setting.UUID) == "" {
		setting.UUID = uuid.NewString()
	}
	if strings.TrimSpace(setting.OriginNode) == "" {
		setting.OriginNode = "local"
	}
	return nil
}
