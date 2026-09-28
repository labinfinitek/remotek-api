package service

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

type PeerService struct {
}

// FindById restituisce il dispositivo con id id; ErrNotFound se non c'e'.
func (ps *PeerService) FindById(id string) (*model.Peer, error) {
	p := &model.Peer{}
	if err := DB.Where("id = ?", id).First(p).Error; err != nil {
		return nil, fmt.Errorf("dispositivo per id: %w", nonTrovato(err))
	}
	return p, nil
}
func (ps *PeerService) InfoByRowId(id uint) *model.Peer {
	p := &model.Peer{}
	DB.Where("row_id = ?", id).First(p)
	return p
}

// UuidBindUserId lega all'utente userId il dispositivo con uuid. Se il
// dispositivo non c'e' non fa niente: lo crea il suo /api/sysinfo.
func (ps *PeerService) UuidBindUserId(uuid string, userId uint) error {
	peer := &model.Peer{}
	err := DB.Where("uuid = ?", uuid).First(peer).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lettura del dispositivo: %w", err)
	}
	peer.UserId = userId
	if err := ps.Update(peer); err != nil {
		return fmt.Errorf("aggiornamento del dispositivo: %w", err)
	}
	return nil
}

// UuidUnbindUserId scollega dall'utente userId il suo dispositivo con uuid,
// se c'e'. Serve al logout.
func (ps *PeerService) UuidUnbindUserId(uuid string, userId uint) error {
	peer := &model.Peer{}
	err := DB.Where("uuid = ? and user_id = ?", uuid, userId).First(peer).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lettura del dispositivo: %w", err)
	}
	if err := DB.Model(peer).Update("user_id", 0).Error; err != nil {
		return fmt.Errorf("aggiornamento del dispositivo: %w", err)
	}
	return nil
}

// EraseUserId 清除用户id, 用于用户删除
func (ps *PeerService) EraseUserId(userId uint) error {
	return DB.Model(&model.Peer{}).Where("user_id = ?", userId).Update("user_id", 0).Error
}

// ListByUserIds 根据用户id取列表
func (ps *PeerService) ListByUserIds(userIds []uint, page, pageSize uint) (res *model.PeerList) {
	res = &model.PeerList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Peer{})
	tx.Where("user_id in (?)", userIds)
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.Peers)
	return
}

func (ps *PeerService) List(page, pageSize uint, where func(tx *gorm.DB)) (res *model.PeerList) {
	res = &model.PeerList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Peer{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.Peers)
	return
}

// Create 创建
func (ps *PeerService) Create(u *model.Peer) error {
	res := DB.Create(u).Error
	return res
}

// Delete 删除, 同时也应该删除token
func (ps *PeerService) Delete(u *model.Peer) error {
	uuid := u.Uuid
	err := DB.Delete(u).Error
	if err != nil {
		return err
	}
	// 删除token
	return AllService.UserService.FlushTokenByUuid(uuid)
}

// GetUuidListByIDs 根据ids获取uuid列表
func (ps *PeerService) GetUuidListByIDs(ids []uint) ([]string, error) {
	var uuids []string
	err := DB.Model(&model.Peer{}).
		Where("row_id in (?)", ids).
		Pluck("uuid", &uuids).Error
	// 过滤uuids中的空字符串
	var newUuids []string
	for _, uuid := range uuids {
		if uuid != "" {
			newUuids = append(newUuids, uuid)
		}
	}
	return newUuids, err
}

// BatchDelete cancella i dispositivi ids e i token di sessione dei loro uuid,
// in una transazione che su errore o panic si annulla. Se gli uuid non si
// leggono non cancella niente: i token dei dispositivi cancellati
// resterebbero validi.
func (ps *PeerService) BatchDelete(ids []uint) error {
	uuids, err := ps.GetUuidListByIDs(ids)
	if err != nil {
		return fmt.Errorf("uuid dei dispositivi da cancellare: %w", err)
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("row_id in (?)", ids).Delete(&model.Peer{}).Error; err != nil {
			return fmt.Errorf("dispositivi: %w", err)
		}
		if err := tx.Where("device_uuid in (?)", uuids).Delete(&model.UserToken{}).Error; err != nil {
			return fmt.Errorf("token di sessione dei dispositivi: %w", err)
		}
		return nil
	})
}

// Update 更新
func (ps *PeerService) Update(u *model.Peer) error {
	return DB.Model(u).Updates(u).Error
}
