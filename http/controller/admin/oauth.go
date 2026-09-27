package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/request/admin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

type Oauth struct {
}

// Info risponde, per il codice nella query, con i soli campi della voce in
// cache che le pagine oauth del pannello mostrano: id e device_name
// (login.vue), op (bind.vue). Verifier PKCE, nonce e dati dell'utente del
// provider non escono.
func (o *Oauth) Info(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	v := service.AllService.OauthService.GetOauthCache(code)
	if v == nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
		return
	}
	response.Success(c, gin.H{"id": v.Id, "device_name": v.DeviceName, "op": v.Op})
}

func (o *Oauth) ToBind(c *gin.Context) {
	f := &admin.BindOauthForm{}
	err := c.ShouldBindJSON(f)
	if err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	u := service.AllService.UserService.CurUser(c)

	utr := service.AllService.UserService.UserThirdInfo(u.Id, f.Op)
	if utr.Id > 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "OauthHasBindOtherUser"))
		return
	}

	err, state, verifier, nonce, url := service.AllService.OauthService.BeginAuth(f.Op)
	if err != nil {
		response.ErrorErr(c, "SystemError", err)
		return
	}

	service.AllService.OauthService.SetOauthCache(state, &service.OauthCacheItem{
		Action:   service.OauthActionTypeBind,
		Op:       f.Op,
		UserId:   u.Id,
		Verifier: verifier,
		Nonce:    nonce,
	}, 5*60)

	response.Success(c, gin.H{
		"code": state,
		"url":  url,
	})
}

// Confirm lega all'utente del pannello il login webauth del codice: il
// dispositivo che lo ha chiesto riceve il token di questo utente. Accetta
// solo un login webauth non ancora confermato e solo con app.web-sso acceso;
// un login OIDC, che deve passare dal provider, o un'associazione ricevono
// la risposta del codice scaduto. login.vue non legge i dati della risposta.
func (o *Oauth) Confirm(c *gin.Context) {
	j := &admin.OauthConfirmForm{}
	err := c.ShouldBindJSON(j)
	if err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	if j.Code == "" {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	v := service.AllService.OauthService.GetOauthCache(j.Code)
	if v == nil || !global.Config.App.WebSso || v.Op != model.OauthTypeWebauth ||
		v.Action != service.OauthActionTypeLogin || v.UserId != 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "OauthExpired"))
		return
	}
	u := service.AllService.UserService.CurUser(c)
	v.UserId = u.Id
	service.AllService.OauthService.SetOauthCache(j.Code, v, 0)
	response.Success(c, nil)
}

// BindConfirm associa all'utente del pannello l'account del provider di un
// login OIDC che il provider ha gia' autenticato ma che non e' ancora di
// nessun utente (OauthCallback, autoregistrazione spenta, rimanda a
// /_admin/#/oauth/bind/<code>), e lega il login a questo utente. Senza
// OpenId il provider non ha risposto: un codice appena creato da
// /api/oidc/auth, o un login gia' legato, riceve la risposta del codice
// scaduto. bind.vue legge solo device_type.
func (o *Oauth) BindConfirm(c *gin.Context) {
	j := &admin.OauthConfirmForm{}
	err := c.ShouldBindJSON(j)
	if err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	if j.Code == "" {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	oauthService := service.AllService.OauthService
	oauthCache := oauthService.GetOauthCache(j.Code)
	if oauthCache == nil || oauthCache.Action != service.OauthActionTypeLogin ||
		oauthCache.OpenId == "" || oauthCache.UserId != 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "OauthExpired"))
		return
	}
	oauthUser := oauthCache.ToOauthUser()
	user := service.AllService.UserService.CurUser(c)
	err = oauthService.BindOauthUser(user.Id, oauthUser, oauthCache.Op)
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "BindFail"))
		return
	}

	oauthCache.UserId = user.Id
	oauthService.SetOauthCache(j.Code, oauthCache, 0)
	response.Success(c, gin.H{"device_type": oauthCache.DeviceType})
}

func (o *Oauth) Unbind(c *gin.Context) {
	f := &admin.UnBindOauthForm{}
	err := c.ShouldBindJSON(f)
	if err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	u := service.AllService.UserService.CurUser(c)
	utr := service.AllService.UserService.UserThirdInfo(u.Id, f.Op)
	if utr.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
		return
	}
	err = service.AllService.OauthService.UnBindOauthUser(u.Id, f.Op)
	if err != nil {
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	response.Success(c, nil)
}

// Detail Oauth
// @Tags Oauth
// @Summary Oauth详情
// @Description Oauth详情
// @Accept  json
// @Produce  json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=model.Oauth}
// @Failure 500 {object} response.Response
// @Router /admin/oauth/detail/{id} [get]
// @Security token
func (o *Oauth) Detail(c *gin.Context) {
	id := c.Param("id")
	iid, _ := strconv.Atoi(id)
	u := service.AllService.OauthService.InfoById(uint(iid))
	if u.Id > 0 {
		response.Success(c, u)
		return
	}
	response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
}

// Create 创建Oauth
// @Tags Oauth
// @Summary 创建Oauth
// @Description 创建Oauth
// @Accept  json
// @Produce  json
// @Param body body admin.OauthForm true "Oauth信息"
// @Success 200 {object} response.Response{data=model.Oauth}
// @Failure 500 {object} response.Response
// @Router /admin/oauth/create [post]
// @Security token
func (o *Oauth) Create(c *gin.Context) {
	f := &admin.OauthForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	u := f.ToOauth()
	err := u.FormatOauthInfo()
	if err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	ex := service.AllService.OauthService.InfoByOp(u.Op)
	if ex.Id > 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ItemExists"))
		return
	}
	err = service.AllService.OauthService.Create(u)
	if err != nil {
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	response.Success(c, nil)
}

// List 列表
// @Tags Oauth
// @Summary Oauth列表
// @Description Oauth列表
// @Accept  json
// @Produce  json
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Success 200 {object} response.Response{data=model.OauthList}
// @Failure 500 {object} response.Response
// @Router /admin/oauth/list [get]
// @Security token
func (o *Oauth) List(c *gin.Context) {
	query := &admin.PageQuery{}
	if err := c.ShouldBindQuery(query); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	res := service.AllService.OauthService.List(query.Page, query.PageSize, nil)
	response.Success(c, res)
}

// Update 编辑
// @Tags Oauth
// @Summary Oauth编辑
// @Description Oauth编辑
// @Accept  json
// @Produce  json
// @Param body body admin.OauthForm true "Oauth信息"
// @Success 200 {object} response.Response{data=model.OauthList}
// @Failure 500 {object} response.Response
// @Router /admin/oauth/update [post]
// @Security token
func (o *Oauth) Update(c *gin.Context) {
	f := &admin.OauthForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	if f.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	u := f.ToOauth()
	err := service.AllService.OauthService.Update(u)
	if err != nil {
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	response.Success(c, nil)
}

// Delete 删除
// @Tags Oauth
// @Summary Oauth删除
// @Description Oauth删除
// @Accept  json
// @Produce  json
// @Param body body admin.OauthForm true "Oauth信息"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/oauth/delete [post]
// @Security token
func (o *Oauth) Delete(c *gin.Context) {
	f := &admin.OauthForm{}
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
	u := service.AllService.OauthService.InfoById(f.Id)
	if u.Id > 0 {
		err := service.AllService.OauthService.Delete(u)
		if err == nil {
			response.Success(c, nil)
			return
		}
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
}
