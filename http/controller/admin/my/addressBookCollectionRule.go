package my

import (
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/request/admin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

type AddressBookCollectionRule struct {
}

// List 列表
// @Tags 我的地址簿规则
// @Summary 地址簿规则列表
// @Description 地址簿规则列表
// @Accept  json
// @Produce  json
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Param is_my query int false "是否是我的"
// @Param user_id query int false "用户id"
// @Param collection_id query int false "地址簿集合id"
// @Success 200 {object} response.Response{data=model.AddressBookCollectionList}
// @Failure 500 {object} response.Response
// @Router /admin/my/address_book_collection_rule/list [get]
// @Security token
func (abcr *AddressBookCollectionRule) List(c *gin.Context) {
	query := &admin.AddressBookCollectionRuleQuery{}
	if err := c.ShouldBindQuery(query); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	u := service.AllService.UserService.CurUser(c)
	query.UserId = int(u.Id)

	res, err := service.AllService.AddressBookService.ListRules(query.Page, query.PageSize, func(tx *gorm.DB) {
		tx.Where("user_id = ?", query.UserId)
		if query.CollectionId > 0 {
			tx.Where("collection_id = ?", query.CollectionId)
		}
	})
	if err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	response.Success(c, res)
}

// Create 创建地址簿规则
// @Tags 我的地址簿规则
// @Summary 创建地址簿规则
// @Description 创建地址簿规则
// @Accept  json
// @Produce  json
// @Param body body model.AddressBookCollectionRule true "地址簿规则信息"
// @Success 200 {object} response.Response{data=model.AddressBookCollection}
// @Failure 500 {object} response.Response
// @Router /admin/my/address_book_collection_rule/create [post]
// @Security token
func (abcr *AddressBookCollectionRule) Create(c *gin.Context) {
	f := &model.AddressBookCollectionRule{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	if f.Type != model.ShareAddressBookRuleTypePersonal && f.Type != model.ShareAddressBookRuleTypeGroup {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	// t := f.ToAddressBookCollection()
	t := f
	u := service.AllService.UserService.CurUser(c)
	t.UserId = u.Id
	if err := abcr.CheckForm(u, t); err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	err := service.AllService.AddressBookService.CreateRule(t)
	if err != nil {
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	response.Success(c, nil)
}

// CheckForm controlla la regola t dell'utente u prima di salvarla, come
// CheckForm del pannello di amministrazione.
func (abcr *AddressBookCollectionRule) CheckForm(u *model.User, t *model.AddressBookCollectionRule) error {
	if t.UserId != u.Id {
		return errors.New("NoAccess")
	}
	switch sua, err := service.AllService.AddressBookService.CheckCollectionOwner(t.UserId, t.CollectionId); {
	case err != nil:
		return err
	case !sua:
		return errors.New("ParamsError")
	}

	// check to_id
	switch t.Type {
	case model.ShareAddressBookRuleTypePersonal:
		if t.ToId == t.UserId {
			return errors.New("CannotShareToSelf")
		}
		tou := service.AllService.UserService.InfoById(t.ToId)
		if tou.Id == 0 {
			return errors.New("ItemNotFound")
		}
		// 非管理员不能分享给非本组织用户
		// if tou.GroupId != u.GroupId {
		//	return "NoAccess", false
		// }
	case model.ShareAddressBookRuleTypeGroup:
		// 非管理员不能分享给其他组
		// if t.ToId != u.GroupId {
		//	return "NoAccess", false
		// }

		tog := service.AllService.GroupService.InfoById(t.ToId)
		if tog.Id == 0 {
			return errors.New("ItemNotFound")
		}
	default:
		return errors.New("ParamsError")
	}
	// un'altra regola con tipo, destinatario e collezione di t e' un doppione
	switch ex, err := service.AllService.AddressBookService.RuleInfoByToIdAndCid(t.Type, t.ToId, t.CollectionId); {
	case errors.Is(err, service.ErrNotFound):
		return nil
	case err != nil:
		return err
	case ex.Id != t.Id:
		return errors.New("ItemExists")
	}
	return nil
}

// Update 编辑
// @Tags 我的地址簿规则
// @Summary 地址簿规则编辑
// @Description 地址簿规则编辑
// @Accept  json
// @Produce  json
// @Param body body model.AddressBookCollectionRule true "地址簿规则信息"
// @Success 200 {object} response.Response{data=model.AddressBookCollection}
// @Failure 500 {object} response.Response
// @Router /admin/my/address_book_collection_rule/update [post]
// @Security token
func (abcr *AddressBookCollectionRule) Update(c *gin.Context) {
	f := &model.AddressBookCollectionRule{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	if f.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	u := service.AllService.UserService.CurUser(c)

	ex, err := service.AllService.AddressBookService.RuleInfoById(f.Id)
	if err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	if ex.UserId != u.Id {
		response.Fail(c, 101, response.TranslateMsg(c, "NoAccess"))
		return
	}
	t := f
	if err = abcr.CheckForm(u, t); err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	err = service.AllService.AddressBookService.UpdateRule(t)
	if err != nil {
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	response.Success(c, nil)
}

// Delete 删除
// @Tags 我的地址簿规则
// @Summary 地址簿规则删除
// @Description 地址簿规则删除
// @Accept  json
// @Produce  json
// @Param body body model.AddressBookCollectionRule true "地址簿规则信息"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/my/address_book_collection_rule/delete [post]
// @Security token
func (abcr *AddressBookCollectionRule) Delete(c *gin.Context) {
	f := &model.AddressBookCollectionRule{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	id := f.Id
	errList := global.Validator.ValidVar(c, id, "required,gt=0")
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	ex, err := service.AllService.AddressBookService.RuleInfoById(f.Id)
	if err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	u := service.AllService.UserService.CurUser(c)
	if ex.UserId != u.Id {
		response.Fail(c, 101, response.TranslateMsg(c, "NoAccess"))
		return
	}

	err = service.AllService.AddressBookService.DeleteRule(ex)
	if err == nil {
		response.Success(c, nil)
		return
	}
	response.FailErr(c, 101, "OperationFailed", err)
}
