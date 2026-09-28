package service

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

type LoginLogService struct {
}

// InfoById restituisce la voce id del registro dei login; ErrNotFound se non
// c'e'.
func (us *LoginLogService) InfoById(id uint) (*model.LoginLog, error) {
	u := &model.LoginLog{}
	if err := DB.Where("id = ?", id).First(u).Error; err != nil {
		return nil, fmt.Errorf("voce %d del registro dei login: %w", id, nonTrovato(err))
	}
	return u, nil
}

// List restituisce la pagina page di pageSize voci del registro dei login,
// filtrate da where se non e' nil.
func (us *LoginLogService) List(page, pageSize uint, where func(tx *gorm.DB)) (*model.LoginLogList, error) {
	res := &model.LoginLogList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.LoginLog{})
	if where != nil {
		where(tx)
	}
	if err := tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio del registro dei login: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err := tx.Find(&res.LoginLogs).Error; err != nil {
		return nil, fmt.Errorf("registro dei login: %w", err)
	}
	return res, nil
}

// Create 创建
func (us *LoginLogService) Create(u *model.LoginLog) error {
	res := DB.Create(u).Error
	return res
}
func (us *LoginLogService) Delete(u *model.LoginLog) error {
	return DB.Delete(u).Error
}

// Update 更新
func (us *LoginLogService) Update(u *model.LoginLog) error {
	return DB.Model(u).Updates(u).Error
}

func (us *LoginLogService) BatchDelete(ids []uint) error {
	return DB.Where("id in (?)", ids).Delete(&model.LoginLog{}).Error
}

func (us *LoginLogService) SoftDelete(l *model.LoginLog) error {
	l.IsDeleted = model.IsDeletedYes
	return us.Update(l)
}

func (us *LoginLogService) BatchSoftDelete(uid uint, ids []uint) error {
	return DB.Model(&model.LoginLog{}).Where("user_id = ? and id in (?)", uid, ids).Update("is_deleted", model.IsDeletedYes).Error
}
