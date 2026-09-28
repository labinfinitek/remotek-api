package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

type AddressBookService struct {
}

// InfoByUserIdAndIdAndCid restituisce la voce id della collezione cid
// dell'utente userid; ErrNotFound se non c'e'.
func (s *AddressBookService) InfoByUserIdAndIdAndCid(userid uint, id string, cid uint) (*model.AddressBook, error) {
	p := &model.AddressBook{}
	if err := DB.Where("user_id = ? and id = ? and collection_id = ?", userid, id, cid).First(p).Error; err != nil {
		return nil, fmt.Errorf("voce della collezione %d dell'utente %d: %w", cid, userid, nonTrovato(err))
	}
	return p, nil
}
func (s *AddressBookService) InfoByRowId(id uint) *model.AddressBook {
	p := &model.AddressBook{}
	DB.Where("row_id = ?", id).First(p)
	return p
}

// AddAddressBook aggiunge la voce ab alla rubrica.
func (s *AddressBookService) AddAddressBook(ab *model.AddressBook) error {
	return DB.Create(ab).Error
}

// UpdateAddressBook porta la rubrica dell'utente a abs: aggiunge le voci
// nuove, aggiorna quelle che ci sono e cancella le altre, in una transazione
// che su errore o panic si annulla. Tocca solo la rubrica personale
// (collezione 0), la sola che GET /api/ab manda al client legacy.
func (s *AddressBookService) UpdateAddressBook(abs []*model.AddressBook, userId uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return s.updateAddressBook(tx, abs, userId)
	})
}

// updateAddressBook e' UpdateAddressBook dentro la transazione tx. Ogni query
// passa da tx: con una connessione sola, una query su DB aspetterebbe per
// sempre la connessione che tx tiene.
func (s *AddressBookService) updateAddressBook(tx *gorm.DB, abs []*model.AddressBook, userId uint) error {
	// 1. 获取数据库中的数据
	var dbABs []*model.AddressBook
	if err := tx.Where("user_id = ? and collection_id = ?", userId, 0).Find(&dbABs).Error; err != nil {
		return fmt.Errorf("lettura della rubrica: %w", err)
	}
	// 2. 比较peers和数据库中的数据
	// 2.1 获取peers中的id
	aBIds := make(map[string]*model.AddressBook)
	for _, ab := range abs {
		aBIds[ab.Id] = ab
	}
	// 2.2 获取数据库中的id
	dbABIds := make(map[string]*model.AddressBook)
	for _, dbAb := range dbABs {
		dbABIds[dbAb.Id] = dbAb
	}
	// 2.3 比较peers和数据库中的数据
	for id, ab := range aBIds {
		dbAB, ok := dbABIds[id]
		ab.UserId = userId
		ab.CollectionId = 0
		if !ok {
			// 添加
			if ab.Platform == "" || ab.Username == "" || ab.Hostname == "" {
				peer := &model.Peer{}
				switch err := tx.Where("id = ?", ab.Id).First(peer).Error; {
				case err == nil:
					ab.Platform = s.PlatformFromOs(peer.Os)
					ab.Username = peer.Username
					ab.Hostname = peer.Hostname
				case !errors.Is(err, gorm.ErrRecordNotFound):
					return fmt.Errorf("dispositivo della voce nuova: %w", err)
				}
			}
			if err := tx.Create(ab).Error; err != nil {
				return fmt.Errorf("voce nuova della rubrica: %w", err)
			}
		} else {
			// 更新
			if err := tx.Model(&model.AddressBook{}).Where("row_id = ?", dbAB.RowId).Updates(ab).Error; err != nil {
				return fmt.Errorf("aggiornamento della rubrica: %w", err)
			}
		}
	}
	// 2.4 删除
	for id, dbAB := range dbABIds {
		_, ok := aBIds[id]
		if !ok {
			if err := tx.Delete(dbAB).Error; err != nil {
				return fmt.Errorf("cancellazione dalla rubrica: %w", err)
			}
		}
	}
	return nil
}

func (s *AddressBookService) List(page, pageSize uint, where func(tx *gorm.DB)) (res *model.AddressBookList, err error) {
	res = &model.AddressBookList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AddressBook{})
	if where != nil {
		where(tx)
	}
	if err = tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio delle voci della rubrica: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err = tx.Find(&res.AddressBooks).Error; err != nil {
		return nil, fmt.Errorf("voci della rubrica: %w", err)
	}
	return res, nil
}

func (s *AddressBookService) FromPeer(peer *model.Peer) (a *model.AddressBook) {
	a = &model.AddressBook{}
	a.Id = peer.Id
	a.Username = peer.Username
	a.Hostname = peer.Hostname
	a.UserId = peer.UserId
	a.Platform = s.PlatformFromOs(peer.Os)
	return a
}

// Create 创建
func (s *AddressBookService) Create(u *model.AddressBook) error {
	res := DB.Create(u).Error
	return res
}
func (s *AddressBookService) Delete(u *model.AddressBook) error {
	return DB.Delete(u).Error
}

// Update 更新
func (s *AddressBookService) Update(u *model.AddressBook) error {
	return DB.Model(u).Updates(u).Error
}

// UpdateByMap 更新
func (s *AddressBookService) UpdateByMap(u *model.AddressBook, data map[string]interface{}) error {
	return DB.Model(u).Updates(data).Error
}

// UpdateAll 更新
func (s *AddressBookService) UpdateAll(u *model.AddressBook) error {
	return DB.Model(u).Select("*").Omit("created_at").Updates(u).Error
}

// PlatformFromOs restituisce la piattaforma della rubrica (Android, Windows,
// Linux, Mac OS) del sistema operativo os di un dispositivo, o "".
func (s *AddressBookService) PlatformFromOs(os string) string {
	if strings.Contains(os, "Android") || strings.Contains(os, "android") {
		return "Android"
	}
	if strings.Contains(os, "Windows") || strings.Contains(os, "windows") {
		return "Windows"
	}
	if strings.Contains(os, "Linux") || strings.Contains(os, "linux") {
		return "Linux"
	}
	if strings.Contains(os, "mac") || strings.Contains(os, "Mac") {
		return "Mac OS"
	}
	return ""
}
func (s *AddressBookService) ListByUserIdAndCollectionId(userId, cid, page, pageSize uint) (*model.AddressBookList, error) {
	return s.List(page, pageSize, func(tx *gorm.DB) {
		tx.Where("user_id = ? and collection_id = ?", userId, cid)
	})
}
func (s *AddressBookService) ListCollection(page, pageSize uint, where func(tx *gorm.DB)) (res *model.AddressBookCollectionList, err error) {
	res = &model.AddressBookCollectionList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AddressBookCollection{})
	if where != nil {
		where(tx)
	}
	if err = tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio delle collezioni: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err = tx.Find(&res.AddressBookCollection).Error; err != nil {
		return nil, fmt.Errorf("collezioni: %w", err)
	}
	return res, nil
}
func (s *AddressBookService) ListCollectionByIds(ids []uint) (res []*model.AddressBookCollection, err error) {
	if err = DB.Where("id in ?", ids).Find(&res).Error; err != nil {
		return nil, fmt.Errorf("collezioni per id: %w", err)
	}
	return res, nil
}

func (s *AddressBookService) ListCollectionByUserId(userId uint) (*model.AddressBookCollectionList, error) {
	return s.ListCollection(1, 100, func(tx *gorm.DB) {
		tx.Where("user_id = ?", userId)
	})
}

// CollectionInfoById restituisce la collezione id; ErrNotFound se non c'e'.
func (s *AddressBookService) CollectionInfoById(id uint) (*model.AddressBookCollection, error) {
	p := &model.AddressBookCollection{}
	if err := DB.Where("id = ?", id).First(p).Error; err != nil {
		return nil, fmt.Errorf("collezione %d: %w", id, nonTrovato(err))
	}
	return p, nil
}

func (s *AddressBookService) CollectionReadRules(user *model.User) (res []*model.AddressBookCollectionRule, err error) {
	// personalRules
	var personalRules []*model.AddressBookCollectionRule
	tx2 := DB.Model(&model.AddressBookCollectionRule{})
	if err = tx2.Where("type = ? and to_id = ? and rule > 0", model.ShareAddressBookRuleTypePersonal, user.Id).Find(&personalRules).Error; err != nil {
		return nil, fmt.Errorf("regole per l'utente %d: %w", user.Id, err)
	}
	res = append(res, personalRules...)

	// group
	var groupRules []*model.AddressBookCollectionRule
	tx3 := DB.Model(&model.AddressBookCollectionRule{})
	if err = tx3.Where("type = ? and to_id = ? and rule > 0", model.ShareAddressBookRuleTypeGroup, user.GroupId).Find(&groupRules).Error; err != nil {
		return nil, fmt.Errorf("regole per il gruppo %d: %w", user.GroupId, err)
	}
	res = append(res, groupRules...)
	return res, nil
}

// UserMaxRule restituisce il permesso piu' alto dell'utente user sulla
// rubrica cid dell'utente uid. Se una regola non si legge restituisce 0 e
// l'errore: un permesso che non si legge non si concede.
func (s *AddressBookService) UserMaxRule(user *model.User, uid, cid uint) (int, error) {
	// ismy?
	if user.Id == uid {
		return model.ShareAddressBookRuleRuleFullControl, nil
	}
	massima := 0
	personalRules := &model.AddressBookCollectionRule{}
	tx := DB.Model(personalRules)
	switch err := tx.Where("type = ? and collection_id = ? and to_id = ?", model.ShareAddressBookRuleTypePersonal, cid, user.Id).First(&personalRules).Error; {
	case err == nil:
		massima = personalRules.Rule
		if massima == model.ShareAddressBookRuleRuleFullControl {
			return massima, nil
		}
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return 0, fmt.Errorf("regola dell'utente %d sulla rubrica %d: %w", user.Id, cid, err)
	}

	groupRules := &model.AddressBookCollectionRule{}
	tx2 := DB.Model(groupRules)
	switch err := tx2.Where("type = ? and collection_id = ? and to_id = ?", model.ShareAddressBookRuleTypeGroup, cid, user.GroupId).First(&groupRules).Error; {
	case err == nil:
		if groupRules.Rule > massima {
			massima = groupRules.Rule
		}
		if massima == model.ShareAddressBookRuleRuleFullControl {
			return massima, nil
		}
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return 0, fmt.Errorf("regola del gruppo %d sulla rubrica %d: %w", user.GroupId, cid, err)
	}
	return massima, nil
}

func (s *AddressBookService) CreateCollection(t *model.AddressBookCollection) error {
	return DB.Create(t).Error
}

func (s *AddressBookService) UpdateCollection(t *model.AddressBookCollection) error {
	return DB.Model(t).Updates(t).Error
}

// DeleteCollection cancella le regole e le voci della collezione t, poi t,
// in una transazione che su errore o panic si annulla.
func (s *AddressBookService) DeleteCollection(t *model.AddressBookCollection) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("collection_id = ?", t.Id).Delete(&model.AddressBookCollectionRule{}).Error; err != nil {
			return fmt.Errorf("regole della collezione: %w", err)
		}
		if err := tx.Where("collection_id = ?", t.Id).Delete(&model.AddressBook{}).Error; err != nil {
			return fmt.Errorf("voci della collezione: %w", err)
		}
		if err := tx.Delete(t).Error; err != nil {
			return fmt.Errorf("collezione: %w", err)
		}
		return nil
	})
}

func (s *AddressBookService) RuleInfoById(u uint) *model.AddressBookCollectionRule {
	p := &model.AddressBookCollectionRule{}
	DB.Where("id = ?", u).First(p)
	return p
}
func (s *AddressBookService) RulePersonalInfoByToIdAndCid(toid, cid uint) *model.AddressBookCollectionRule {
	return s.RuleInfoByToIdAndCid(model.ShareAddressBookRuleTypePersonal, toid, cid)
}
func (s *AddressBookService) RuleInfoByToIdAndCid(t int, toid, cid uint) *model.AddressBookCollectionRule {
	p := &model.AddressBookCollectionRule{}
	DB.Where("type = ? and to_id = ? and collection_id = ?", t, toid, cid).First(p)
	return p
}
func (s *AddressBookService) CreateRule(t *model.AddressBookCollectionRule) error {
	return DB.Create(t).Error
}

func (s *AddressBookService) ListRules(page uint, size uint, f func(tx *gorm.DB)) *model.AddressBookCollectionRuleList {
	res := &model.AddressBookCollectionRuleList{}
	res.Page = int64(page)
	res.PageSize = int64(size)
	tx := DB.Model(&model.AddressBookCollectionRule{})
	if f != nil {
		f(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, size))
	tx.Find(&res.AddressBookCollectionRule)
	return res
}

func (s *AddressBookService) UpdateRule(t *model.AddressBookCollectionRule) error {
	return DB.Model(t).Updates(t).Error
}

func (s *AddressBookService) DeleteRule(t *model.AddressBookCollectionRule) error {
	return DB.Delete(t).Error
}

// CheckCollectionOwner dice se la collezione cid e' dell'utente uid. Una
// collezione che non si legge vale come non sua: il pannello rifiuta, ma
// l'errore non arriva ancora al log.
func (s *AddressBookService) CheckCollectionOwner(uid uint, cid uint) bool {
	p, err := s.CollectionInfoById(cid)
	return err == nil && p.UserId == uid
}

func (s *AddressBookService) BatchUpdateTags(abs []*model.AddressBook, tags []string) error {
	ids := make([]uint, 0)
	for _, ab := range abs {
		ids = append(ids, ab.RowId)
	}
	tagsv, _ := json.Marshal(tags)
	return DB.Model(&model.AddressBook{}).Where("row_id in ?", ids).Update("tags", tagsv).Error
}
