package service

import (
	"errors"
	"fmt"
	"time"

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
// Con l'indice unico su peers.id, creato all'avvio, un ID ha una riga sola.
// Su un database vecchio con righe doppie l'indice non c'e' (l'avvio lo
// scrive nel log) e vale quella che FindById restituisce, la prima per
// row_id, finche' le altre non si cancellano dal pannello.
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

// Firmato e' l'esito di RiconosciFirmato.
type Firmato struct {
	Peer  *model.Peer
	Esito Riconoscimento
	// Rifiuto, se non vuoto, e' il motivo per cui la firma non basta: la
	// richiesta si rifiuta come per un altro dispositivo.
	Rifiuto string
	// ChiaveDiversa: uuid giusto e firma valida, ma con una pk diversa da
	// quella salvata (PC reinstallato o chiave registrata da altri).
	ChiaveDiversa bool
	// Chiave e' la pk da salvare col sysinfo, vuota se non c'e' da salvarne.
	Chiave string
	// NonRegistrata e' il motivo per cui una pk arrivata non si salva.
	NonRegistrata string
}

// RiconosciFirmato applica a Riconosci le regole della firma del
// dispositivo (ADR-0023, README "Firma del dispositivo"): un PC con la
// chiave accetta solo richieste firmate con quella; uno senza resta con le
// regole di ADR-0019, a meno di app.firma-obbligatoria; un sysinfo con pk e
// firma valida registra la chiave di un PC nuovo o con lo stesso uuid che
// non ne ha. Una chiave salvata non cambia mai da qui.
func (ps *PeerService) RiconosciFirmato(id, uuid string, r RichiestaFirmata) (Firmato, error) {
	p, esito, err := ps.Riconosci(id, uuid)
	if err != nil {
		return Firmato{}, err
	}
	f := Firmato{Peer: p, Esito: esito}
	ora := time.Now()
	if p != nil && p.ChiavePubblica != "" {
		f.Rifiuto = r.verifica(p.ChiavePubblica, ora)
		if f.Rifiuto != "" && r.Pk != "" && r.Pk != p.ChiavePubblica && esito == StessoDispositivo {
			f.ChiaveDiversa, _, _ = r.firmaValida(r.Pk)
		}
		return f, nil
	}
	obbligatoria := Config != nil && Config.App.FirmaObbligatoria
	switch {
	case r.Pk == "" && obbligatoria:
		f.Rifiuto = FirmaAssente
		if r.Intestazione != "" {
			f.Rifiuto = SenzaChiave
		}
	case r.Pk == "":
	case !obbligatoria && r.Intestazione == "":
		f.NonRegistrata = FirmaAssente
	default:
		motivo := r.verifica(r.Pk, ora)
		switch {
		case motivo != "" && obbligatoria:
			f.Rifiuto = motivo
		case motivo != "":
			f.NonRegistrata = motivo
		case esito == PcSconosciuto || esito == StessoDispositivo:
			f.Chiave = r.Pk
		default:
			f.NonRegistrata = "uuid diverso da quello salvato"
		}
	}
	return f, nil
}

// RegistraChiave salva la chiave pubblica del PC rowId solo se non ne ha
// gia' una; registrata dice se l'ha salvata.
func (ps *PeerService) RegistraChiave(rowId uint, chiave string) (registrata bool, err error) {
	res := DB.Model(&model.Peer{}).Where("row_id = ? and chiave_pubblica = ''", rowId).Update("chiave_pubblica", chiave)
	if res.Error != nil {
		return false, fmt.Errorf("chiave del dispositivo %d: %w", rowId, res.Error)
	}
	return res.RowsAffected == 1, nil
}

// IndiceIdUnico e' l'indice unico su peers.id: un ID di PC su una riga
// sola, perche' il legame ID-uuid (Riconosci) sia di un dispositivo solo.
const IndiceIdUnico = "idx_peers_id_unico"

// CreaIndiceIdUnico crea l'indice IndiceIdUnico se non c'e' e peers non ha ID
// doppi; creato dice se l'ha creato ora. Se ci sono ID doppi non lo crea e
// restituisce quanti sono; non cancella righe.
func (ps *PeerService) CreaIndiceIdUnico() (creato bool, doppi int64, err error) {
	if DB.Migrator().HasIndex(&model.Peer{}, IndiceIdUnico) {
		return false, 0, nil
	}
	err = DB.Raw("SELECT count(*) FROM (SELECT id FROM peers GROUP BY id HAVING count(*) > 1)").Scan(&doppi).Error
	if err != nil {
		return false, 0, fmt.Errorf("conteggio degli ID doppi in peers: %w", err)
	}
	if doppi > 0 {
		return false, doppi, nil
	}
	if err := DB.Exec("CREATE UNIQUE INDEX " + IndiceIdUnico + " ON peers(id)").Error; err != nil {
		return false, 0, fmt.Errorf("indice %s: %w", IndiceIdUnico, err)
	}
	return true, 0, nil
}

// IdUsato dice se un PC diverso da quello con row_id rowId ha l'ID id; per
// un PC nuovo rowId e' 0.
func (ps *PeerService) IdUsato(id string, rowId uint) (bool, error) {
	var n int64
	if err := DB.Model(&model.Peer{}).Where("id = ? and row_id <> ?", id, rowId).Count(&n).Error; err != nil {
		return false, fmt.Errorf("controllo dell'ID del dispositivo: %w", err)
	}
	return n > 0, nil
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

// Update salva i campi non vuoti di u: serve alle scritture parziali
// (sysinfo, heartbeat, legame con l'utente), che non devono azzerare il resto.
func (ps *PeerService) Update(u *model.Peer) error {
	return DB.Model(u).Updates(u).Error
}

// UpdateDalPannello salva i campi del modulo del pannello (admin.PeerForm),
// anche vuoti, cosi' un nome del PC o un gruppo tolti restano tolti. L'alias
// si scrive solo con conAlias, cioe' se il corpo aveva la chiave: il pannello
// la manda solo se l'alias si e' toccato. L'uuid vuoto non si scrive: il
// modulo lo manda vuoto per non cambiarlo, e scriverlo scioglierebbe il
// legame ID-uuid. L'ID vuoto il validatore di admin.PeerForm lo rifiuta
// prima di qui; il controllo resta per chi chiamasse senza passare dal
// modulo, perche' un PC senza ID non si riconosce piu'. L'utente legato
// (user_id), l'ultimo contatto e l'IP non sono nel modulo e restano quelli di
// prima; username, l'utente del PC, il modulo lo manda e si salva.
func (ps *PeerService) UpdateDalPannello(u *model.Peer, conAlias bool) error {
	campi := []string{"cpu", "hostname", "memory", "os", "username", "version", "group_id"}
	if conAlias {
		campi = append(campi, "alias")
	}
	if u.Id != "" {
		campi = append(campi, "id")
	}
	if u.Uuid != "" {
		campi = append(campi, "uuid")
	}
	if err := DB.Model(u).Select(campi).Updates(u).Error; err != nil {
		return fmt.Errorf("dispositivo %d: %w", u.RowId, err)
	}
	return nil
}
