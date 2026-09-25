package model

import (
	"github.com/lejianwen/rustdesk-api/v2/model/custom_types"
)

type StatusCode int

const (
	COMMON_STATUS_ENABLE   StatusCode = 1 // 通用状态 启用
	COMMON_STATUS_DISABLED StatusCode = 2 // 通用状态 禁用
)

type IdModel struct {
	Id uint `gorm:"primaryKey" json:"id"`
}
type TimeModel struct {
	CreatedAt custom_types.AutoTime `json:"created_at" gorm:"type:timestamp;"`
	UpdatedAt custom_types.AutoTime `json:"updated_at" gorm:"type:timestamp;"`
}

// Pagination e' la paginazione di una lista: pagina, totale e righe per
// pagina.
type Pagination struct {
	Page     int64 `form:"page" json:"page"`
	Total    int64 `form:"total" json:"total"`
	PageSize int64 `form:"page_size" json:"page_size"`
}
