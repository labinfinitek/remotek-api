package service

import (
	"crypto/rand"
	"crypto/sha256"
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

// DeleteAuditConn cancella la connessione u e la sua trascrizione, se ne ha
// una, nella stessa transazione.
func (as *AuditService) DeleteAuditConn(u *model.AuditConn) error {
	return as.BatchDeleteAuditConn([]uint{u.Id})
}

// UpdateAuditConn 更新
func (as *AuditService) UpdateAuditConn(u *model.AuditConn) error {
	return DB.Model(u).Updates(u).Error
}

// InfoByPeerIdAndConnId restituisce la connessione connId piu' recente del
// dispositivo peerId; ErrNotFound se non c'e'. Il client fa ripartire
// conn_id da un valore casuale a ogni avvio del servizio, quindi lo stesso
// conn_id puo' tornare: la sessione in corso e' l'ultima riga.
func (as *AuditService) InfoByPeerIdAndConnId(peerId string, connId int64) (*model.AuditConn, error) {
	res := &model.AuditConn{}
	if err := DB.Where("peer_id = ? and conn_id = ?", peerId, connId).Order("id desc").First(res).Error; err != nil {
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

// BatchDeleteAuditConn cancella le connessioni ids e le loro trascrizioni,
// nella stessa transazione.
func (as *AuditService) BatchDeleteAuditConn(ids []uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("audit_conn_id in (?)", ids).Delete(&model.AuditTerminal{}).Error; err != nil {
			return fmt.Errorf("trascrizioni delle connessioni: %w", err)
		}
		if err := tx.Where("id in (?)", ids).Delete(&model.AuditConn{}).Error; err != nil {
			return fmt.Errorf("connessioni dell'audit: %w", err)
		}
		return nil
	})
}

// ConnTerminale restituisce la connessione della sessione connId del
// dispositivo peerId, la piu' recente con quel conn_id, se e' un terminale;
// ErrNotFound se non c'e' o non e' un terminale.
func (as *AuditService) ConnTerminale(peerId string, connId int64) (*model.AuditConn, error) {
	res, err := as.InfoByPeerIdAndConnId(peerId, connId)
	if err != nil {
		return nil, err
	}
	if res.Type != model.AuditConnTerminale {
		return nil, fmt.Errorf("connessione %d del dispositivo %s di tipo %d: %w", connId, peerId, res.Type, ErrNotFound)
	}
	return res, nil
}

// UltimoBlocco restituisce l'ultimo blocco salvato della trascrizione della
// connessione auditConnId e i byte di tutta la trascrizione; nil e 0 se
// non ce n'e'.
func (as *AuditService) UltimoBlocco(auditConnId uint) (*model.AuditTerminal, int64, error) {
	var totale int64
	tx := DB.Model(&model.AuditTerminal{}).Where("audit_conn_id = ?", auditConnId)
	if err := tx.Select("coalesce(sum(length(data)), 0)").Scan(&totale).Error; err != nil {
		return nil, 0, fmt.Errorf("byte della trascrizione della connessione %d: %w", auditConnId, err)
	}
	var blocchi []*model.AuditTerminal
	err := DB.Where("audit_conn_id = ?", auditConnId).Order("seq desc").Limit(1).Find(&blocchi).Error
	if err != nil {
		return nil, 0, fmt.Errorf("ultimo blocco della trascrizione della connessione %d: %w", auditConnId, err)
	}
	if len(blocchi) == 0 {
		return nil, 0, nil
	}
	return blocchi[0], totale, nil
}

// CreateBlocco salva il blocco b della trascrizione.
func (as *AuditService) CreateBlocco(b *model.AuditTerminal) error {
	if err := DB.Create(b).Error; err != nil {
		return fmt.Errorf("blocco %d della trascrizione della connessione %d: %w", b.Seq, b.AuditConnId, err)
	}
	return nil
}

// HashBlocco restituisce SHA-256(prec || d || data), con d il byte 'i' per
// dir "in" e 'o' per "out": l'hash di un blocco della trascrizione, dato
// quello del blocco prima (32 byte a zero per il primo).
func HashBlocco(prec []byte, dir string, data []byte) []byte {
	msg := make([]byte, 0, len(prec)+1+len(data))
	msg = append(append(append(msg, prec...), dir[0]), data...)
	h := sha256.Sum256(msg)
	return h[:]
}

// BlocchiTerminale restituisce la pagina page di pageSize blocchi della
// trascrizione della connessione auditConnId, in ordine di seq.
func (as *AuditService) BlocchiTerminale(auditConnId uint, page, pageSize uint) (*model.AuditTerminalList, error) {
	res := &model.AuditTerminalList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AuditTerminal{}).Where("audit_conn_id = ?", auditConnId)
	if err := tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio dei blocchi della trascrizione: %w", err)
	}
	if err := tx.Order("seq").Scopes(Paginate(page, pageSize)).Find(&res.Blocchi).Error; err != nil {
		return nil, fmt.Errorf("blocchi della trascrizione: %w", err)
	}
	return res, nil
}

// VerificaTerminale ricalcola dal database la catena della trascrizione
// della connessione auditConnId. Un seq fuori posto, un
// dir sconosciuto, un hash che non torna o un blocco dopo quello con fine
// rendono la catena non integra dal loro seq.
func (as *AuditService) VerificaTerminale(auditConnId uint) (*model.AuditTerminalVerifica, error) {
	rows, err := DB.Model(&model.AuditTerminal{}).Where("audit_conn_id = ?", auditConnId).Order("seq").Rows()
	if err != nil {
		return nil, fmt.Errorf("trascrizione della connessione %d: %w", auditConnId, err)
	}
	defer rows.Close()
	res := &model.AuditTerminalVerifica{Integra: true}
	prec := make([]byte, sha256.Size)
	for rows.Next() {
		b := &model.AuditTerminal{}
		if err := DB.ScanRows(rows, b); err != nil {
			return nil, fmt.Errorf("blocco della trascrizione della connessione %d: %w", auditConnId, err)
		}
		res.Blocchi++
		if res.Integra {
			valido := b.Dir == "in" || b.Dir == "out"
			if valido {
				prec = HashBlocco(prec, b.Dir, b.Data)
			}
			if !valido || b.Seq != res.Blocchi || res.Fine || hex.EncodeToString(prec) != b.Hash {
				res.Integra = false
				res.PrimoErrato = b.Seq
			}
		}
		res.Fine = res.Fine || b.Fine
		res.Hash = b.Hash
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("trascrizione della connessione %d: %w", auditConnId, err)
	}
	return res, nil
}

func (as *AuditService) BatchDeleteAuditFile(ids []uint) error {
	return DB.Where("id in (?)", ids).Delete(&model.AuditFile{}).Error
}
