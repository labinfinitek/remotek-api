package service

import (
	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

type AuditService struct {
}

func (as *AuditService) AuditConnList(page, pageSize uint, where func(tx *gorm.DB)) (res *model.AuditConnList) {
	res = &model.AuditConnList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AuditConn{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.AuditConns)
	return
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
// peerId, vuota se non la legge.
func (as *AuditService) InfoByPeerIdAndConnId(peerId string, connId int64) (res *model.AuditConn) {
	res = &model.AuditConn{}
	DB.Where("peer_id = ? and conn_id = ?", peerId, connId).First(res)
	return
}

// ConnInfoById restituisce la connessione id, vuota se non la legge.
func (as *AuditService) ConnInfoById(id uint) (res *model.AuditConn) {
	res = &model.AuditConn{}
	DB.Where("id = ?", id).First(res)
	return
}

// FileInfoById restituisce il trasferimento di file id, vuoto se non lo
// legge.
func (as *AuditService) FileInfoById(id uint) (res *model.AuditFile) {
	res = &model.AuditFile{}
	DB.Where("id = ?", id).First(res)
	return
}

func (as *AuditService) AuditFileList(page, pageSize uint, where func(tx *gorm.DB)) (res *model.AuditFileList) {
	res = &model.AuditFileList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AuditFile{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.AuditFiles)
	return
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
