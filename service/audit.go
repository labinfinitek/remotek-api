package service

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

type AuditService struct {
}

// AuditConnList restituisce la pagina page di pageSize connessioni
// dell'audit, filtrate da where se non e' nil.
func (as *AuditService) AuditConnList(page, pageSize uint, where func(tx *gorm.DB)) (*model.AuditConnList, error) {
	res := &model.AuditConnList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AuditConn{})
	if where != nil {
		where(tx)
	}
	if err := tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio delle connessioni dell'audit: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err := tx.Find(&res.AuditConns).Error; err != nil {
		return nil, fmt.Errorf("connessioni dell'audit: %w", err)
	}
	return res, nil
}

// CreateAuditConn 创建
func (as *AuditService) CreateAuditConn(u *model.AuditConn) error {
	res := DB.Create(u).Error
	return res
}
func (as *AuditService) DeleteAuditConn(u *model.AuditConn) error {
	return DB.Delete(u).Error
}

// UpdateAuditConn 更新
func (as *AuditService) UpdateAuditConn(u *model.AuditConn) error {
	return DB.Model(u).Updates(u).Error
}

// InfoByPeerIdAndConnId restituisce la connessione connId del dispositivo
// peerId; ErrNotFound se non c'e'.
func (as *AuditService) InfoByPeerIdAndConnId(peerId string, connId int64) (*model.AuditConn, error) {
	res := &model.AuditConn{}
	if err := DB.Where("peer_id = ? and conn_id = ?", peerId, connId).First(res).Error; err != nil {
		return nil, fmt.Errorf("connessione %d del dispositivo %s: %w", connId, peerId, nonTrovato(err))
	}
	return res, nil
}

// ConnInfoById restituisce la connessione id; ErrNotFound se non c'e'.
func (as *AuditService) ConnInfoById(id uint) (*model.AuditConn, error) {
	res := &model.AuditConn{}
	if err := DB.Where("id = ?", id).First(res).Error; err != nil {
		return nil, fmt.Errorf("connessione dell'audit %d: %w", id, nonTrovato(err))
	}
	return res, nil
}

// FileInfoById restituisce il trasferimento di file id; ErrNotFound se non
// c'e'.
func (as *AuditService) FileInfoById(id uint) (*model.AuditFile, error) {
	res := &model.AuditFile{}
	if err := DB.Where("id = ?", id).First(res).Error; err != nil {
		return nil, fmt.Errorf("trasferimento di file %d: %w", id, nonTrovato(err))
	}
	return res, nil
}

// AuditFileList restituisce la pagina page di pageSize trasferimenti di file
// dell'audit, filtrati da where se non e' nil.
func (as *AuditService) AuditFileList(page, pageSize uint, where func(tx *gorm.DB)) (*model.AuditFileList, error) {
	res := &model.AuditFileList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AuditFile{})
	if where != nil {
		where(tx)
	}
	if err := tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio dei trasferimenti di file: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err := tx.Find(&res.AuditFiles).Error; err != nil {
		return nil, fmt.Errorf("trasferimenti di file: %w", err)
	}
	return res, nil
}

// CreateAuditFile salva il trasferimento di file u.
func (as *AuditService) CreateAuditFile(u *model.AuditFile) error {
	res := DB.Create(u).Error
	return res
}
func (as *AuditService) DeleteAuditFile(u *model.AuditFile) error {
	return DB.Delete(u).Error
}

// UpdateAuditFile 更新
func (as *AuditService) UpdateAuditFile(u *model.AuditFile) error {
	return DB.Model(u).Updates(u).Error
}

func (as *AuditService) BatchDeleteAuditConn(ids []uint) error {
	return DB.Where("id in (?)", ids).Delete(&model.AuditConn{}).Error
}

func (as *AuditService) BatchDeleteAuditFile(ids []uint) error {
	return DB.Where("id in (?)", ids).Delete(&model.AuditFile{}).Error
}
