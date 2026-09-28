package service

import (
	"crypto/rand"
	"encoding/hex"
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

// CreateAuditConn salva la connessione u; a quella nuova (azione "new")
// da' un guid casuale di 128 bit, che serve alla nota di fine connessione.
func (as *AuditService) CreateAuditConn(u *model.AuditConn) error {
	if u.Action == model.AuditActionNew && u.Guid == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return fmt.Errorf("guid della connessione: %w", err)
		}
		u.Guid = hex.EncodeToString(b)
	}
	return DB.Create(u).Error
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

// sessioneNonAutorizzata e' il session_id con cui il client 1.4.9 registra
// la connessione prima dell'autorizzazione: il tecnico non lo manda mai, e
// una nota o un guid cercati con questo arriverebbero alle connessioni
// rifiutate o in attesa del clic.
const sessioneNonAutorizzata = "0"

// NotaDiSessione scrive note sulla connessione piu' recente del dispositivo
// peerId con quel sessionId; ErrNotFound se non ce n'e' nessuna o se
// sessionId e' "0". Non crea righe.
func (as *AuditService) NotaDiSessione(peerId, sessionId, note string) error {
	if sessionId == sessioneNonAutorizzata {
		return fmt.Errorf("nota senza sessione del dispositivo %s: %w", peerId, ErrNotFound)
	}
	ex := &model.AuditConn{}
	if err := DB.Where("peer_id = ? and session_id = ?", peerId, sessionId).Order("id desc").First(ex).Error; err != nil {
		return fmt.Errorf("connessione della sessione %s del dispositivo %s: %w", sessionId, peerId, nonTrovato(err))
	}
	if err := DB.Model(ex).Update("note", note).Error; err != nil {
		return fmt.Errorf("nota della connessione %d: %w", ex.Id, err)
	}
	return nil
}

// GuidConnessioneAperta restituisce il guid della connessione aperta piu'
// recente del dispositivo peerId, con quel sessionId e di quel tipo;
// ErrNotFound se non ce n'e' nessuna o se sessionId e' "0".
func (as *AuditService) GuidConnessioneAperta(peerId, sessionId string, tipo int) (string, error) {
	if sessionId == sessioneNonAutorizzata {
		return "", fmt.Errorf("connessione aperta senza sessione del dispositivo %s: %w", peerId, ErrNotFound)
	}
	ex := &model.AuditConn{}
	err := DB.Where("peer_id = ? and session_id = ? and type = ? and close_time = 0", peerId, sessionId, tipo).Order("id desc").First(ex).Error
	if err != nil {
		return "", fmt.Errorf("connessione aperta della sessione %s del dispositivo %s: %w", sessionId, peerId, nonTrovato(err))
	}
	return ex.Guid, nil
}

// NotaPerGuid scrive note sulla connessione col guid guid; ErrNotFound se
// non c'e', anche per il guid vuoto delle connessioni di prima.
func (as *AuditService) NotaPerGuid(guid, note string) error {
	if guid == "" {
		return fmt.Errorf("connessione col guid vuoto: %w", ErrNotFound)
	}
	res := DB.Model(&model.AuditConn{}).Where("guid = ?", guid).Update("note", note)
	if res.Error != nil {
		return fmt.Errorf("nota della connessione per guid: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("connessione per guid: %w", ErrNotFound)
	}
	return nil
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
