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

// ErrDispositivoDiverso dice che l'uuid arrivato con l'ID di un PC non e'
// quello del dispositivo salvato. Il testo e' l'ID del messaggio
// DeviceMismatch: ErrorErr lo trova nella catena dell'errore.
var ErrDispositivoDiverso = errors.New("DeviceMismatch")

// Riconoscimento e' l'esito di Riconosci.
type Riconoscimento int

const (
	// StessoDispositivo: il PC con quell'ID ha quell'uuid.
	StessoDispositivo Riconoscimento = iota + 1
	// UuidDiverso: il PC con quell'ID ha un altro uuid, o nessuno.
	UuidDiverso
	// PcSconosciuto: nessun PC ha quell'ID.
	PcSconosciuto
)

// Riconosci dice se chi manda l'ID id e l'uuid uuid, senza token ne' firma,
// e' il dispositivo salvato con quell'ID (REM-2026-002): le rotte del
// client senza login scrivono solo per StessoDispositivo. Per
// StessoDispositivo e UuidDiverso restituisce anche il PC; un uuid vuoto
// non e' mai lo stesso dispositivo, neanche per un PC senza uuid. Un
// errore del database torna come errore, con esito 0.
//
// Se peers ha piu' righe con lo stesso ID (l'indice non e' unico) vale
// quella che FindById restituisce, la prima per row_id: le righe doppie
// sono un altro compito.
func (ps *PeerService) Riconosci(id, uuid string) (*model.Peer, Riconoscimento, error) {
	p, err := ps.FindById(id)
	if errors.Is(err, ErrNotFound) {
		return nil, PcSconosciuto, nil
	}
	if err != nil {
		return nil, 0, err
	}
	if uuid == "" || p.Uuid != uuid {
		return p, UuidDiverso, nil
	}
	return p, StessoDispositivo, nil
}

// InfoByRowId restituisce il dispositivo con row_id id; ErrNotFound se non
// c'e'.
func (ps *PeerService) InfoByRowId(id uint) (*model.Peer, error) {
	p := &model.Peer{}
	if err := DB.Where("row_id = ?", id).First(p).Error; err != nil {
		return nil, fmt.Errorf("dispositivo %d: %w", id, nonTrovato(err))
	}
	return p, nil
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

// ListByUserIds restituisce la pagina page di pageSize dispositivi degli
// utenti userIds.
func (ps *PeerService) ListByUserIds(userIds []uint, page, pageSize uint) (*model.PeerList, error) {
	res := &model.PeerList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Peer{})
	tx.Where("user_id in (?)", userIds)
	if err := tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio dei dispositivi degli utenti: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err := tx.Find(&res.Peers).Error; err != nil {
		return nil, fmt.Errorf("dispositivi degli utenti: %w", err)
	}
	return res, nil
}

// List restituisce la pagina page di pageSize dispositivi, filtrati da where
// se non e' nil.
func (ps *PeerService) List(page, pageSize uint, where func(tx *gorm.DB)) (*model.PeerList, error) {
	res := &model.PeerList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Peer{})
	if where != nil {
		where(tx)
	}
	if err := tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio dei dispositivi: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err := tx.Find(&res.Peers).Error; err != nil {
		return nil, fmt.Errorf("dispositivi: %w", err)
	}
	return res, nil
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
