package service

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

type TagService struct {
}

// InfoByUserIdAndNameAndCollectionId restituisce il tag name della
// collezione cid dell'utente userid; ErrNotFound se non c'e'.
func (s *TagService) InfoByUserIdAndNameAndCollectionId(userid uint, name string, cid uint) (*model.Tag, error) {
	p := &model.Tag{}
	if err := DB.Where("user_id = ? and name = ? and collection_id = ?", userid, name, cid).First(p).Error; err != nil {
		return nil, fmt.Errorf("tag della collezione %d dell'utente %d: %w", cid, userid, nonTrovato(err))
	}
	return p, nil
}

func (s *TagService) ListByUserIdAndCollectionId(userId, cid uint) (*model.TagList, error) {
	return s.List(1, 1000, func(tx *gorm.DB) {
		tx.Where("user_id = ? and collection_id = ?", userId, cid)
		tx.Order("name asc")
	})
}

// UpdateTags porta i tag dell'utente a tags (nome e colore): aggiunge quelli
// nuovi, aggiorna i colori e cancella gli altri, in una transazione che su
// errore o panic si annulla. Tocca solo i tag della rubrica personale
// (collezione 0).
func (s *TagService) UpdateTags(userId uint, tags map[string]uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return updateTags(tx, userId, tags)
	})
}

// updateTags e' UpdateTags dentro la transazione tx.
func updateTags(tx *gorm.DB, userId uint, tags map[string]uint) error {
	// 先查询所有tag
	var allTags []*model.Tag
	if err := tx.Where("user_id = ? and collection_id = ?", userId, 0).Find(&allTags).Error; err != nil {
		return fmt.Errorf("lettura dei tag: %w", err)
	}
	for _, t := range allTags {
		if _, ok := tags[t.Name]; !ok {
			// 删除
			if err := tx.Delete(t).Error; err != nil {
				return fmt.Errorf("cancellazione di un tag: %w", err)
			}
		} else {
			if tags[t.Name] != t.Color {
				// 更新
				t.Color = tags[t.Name]
				if err := tx.Save(t).Error; err != nil {
					return fmt.Errorf("colore di un tag: %w", err)
				}
			}
			// 移除
			delete(tags, t.Name)
		}
	}
	// 新增
	for tag, color := range tags {
		t := &model.Tag{}
		t.Name = tag
		t.Color = color
		t.UserId = userId
		if err := tx.Create(t).Error; err != nil {
			return fmt.Errorf("tag nuovo: %w", err)
		}
	}
	return nil
}

// InfoById 根据用户id取用户信息
func (s *TagService) InfoById(id uint) *model.Tag {
	u := &model.Tag{}
	DB.Where("id = ?", id).First(u)
	return u
}

func (s *TagService) List(page, pageSize uint, where func(tx *gorm.DB)) (res *model.TagList, err error) {
	res = &model.TagList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Tag{})
	if where != nil {
		where(tx)
	}
	if err = tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio dei tag: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err = tx.Find(&res.Tags).Error; err != nil {
		return nil, fmt.Errorf("tag: %w", err)
	}
	return res, nil
}

// Create 创建
func (s *TagService) Create(u *model.Tag) error {
	res := DB.Create(u).Error
	return res
}
func (s *TagService) Delete(u *model.Tag) error {
	return DB.Delete(u).Error
}

// Update 更新
func (s *TagService) Update(u *model.Tag) error {
	return DB.Model(u).Select("*").Omit("created_at").Updates(u).Error
}
