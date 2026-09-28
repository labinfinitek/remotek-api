package service

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

type GroupService struct {
}

// InfoById restituisce il gruppo di utenti id; ErrNotFound se non c'e'.
func (us *GroupService) InfoById(id uint) (*model.Group, error) {
	u := &model.Group{}
	if err := DB.Where("id = ?", id).First(u).Error; err != nil {
		return nil, fmt.Errorf("gruppo %d: %w", id, nonTrovato(err))
	}
	return u, nil
}

// List restituisce la pagina page di pageSize gruppi di utenti, filtrati da
// where se non e' nil.
func (us *GroupService) List(page, pageSize uint, where func(tx *gorm.DB)) (*model.GroupList, error) {
	res := &model.GroupList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Group{})
	if where != nil {
		where(tx)
	}
	if err := tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio dei gruppi: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err := tx.Find(&res.Groups).Error; err != nil {
		return nil, fmt.Errorf("gruppi: %w", err)
	}
	return res, nil
}

// Create 创建
func (us *GroupService) Create(u *model.Group) error {
	res := DB.Create(u).Error
	return res
}
func (us *GroupService) Delete(u *model.Group) error {
	return DB.Delete(u).Error
}

// Update 更新
func (us *GroupService) Update(u *model.Group) error {
	return DB.Model(u).Updates(u).Error
}

// DeviceGroupInfoById restituisce il gruppo di dispositivi id; ErrNotFound
// se non c'e'.
func (us *GroupService) DeviceGroupInfoById(id uint) (*model.DeviceGroup, error) {
	u := &model.DeviceGroup{}
	if err := DB.Where("id = ?", id).First(u).Error; err != nil {
		return nil, fmt.Errorf("gruppo di dispositivi %d: %w", id, nonTrovato(err))
	}
	return u, nil
}

// DeviceGroupList restituisce la pagina page di pageSize gruppi di
// dispositivi, filtrati da where se non e' nil.
func (us *GroupService) DeviceGroupList(page, pageSize uint, where func(tx *gorm.DB)) (*model.DeviceGroupList, error) {
	res := &model.DeviceGroupList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.DeviceGroup{})
	if where != nil {
		where(tx)
	}
	if err := tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio dei gruppi di dispositivi: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err := tx.Find(&res.DeviceGroups).Error; err != nil {
		return nil, fmt.Errorf("gruppi di dispositivi: %w", err)
	}
	return res, nil
}

func (us *GroupService) DeviceGroupCreate(u *model.DeviceGroup) error {
	res := DB.Create(u).Error
	return res
}
func (us *GroupService) DeviceGroupDelete(u *model.DeviceGroup) error {
	return DB.Delete(u).Error
}

func (us *GroupService) DeviceGroupUpdate(u *model.DeviceGroup) error {
	return DB.Model(u).Updates(u).Error
}
