package admin

import (
	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/request/admin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

type Rustdesk struct {
}

type RustdeskCmd struct {
	Cmd    string `json:"cmd"`
	Option string `json:"option"`
	Target string `json:"target"`
}

func (r *Rustdesk) CmdList(c *gin.Context) {
	q := &admin.PageQuery{}
	if err := c.ShouldBindQuery(q); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	res, err := service.AllService.ServerCmdService.List(q.Page, 9999)
	if err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	// 在列表前添加系统命令
	list := make([]*model.ServerCmd, 0)
	list = append(list, model.SysIdServerCmds...)
	list = append(list, model.SysRelayServerCmds...)
	list = append(list, res.ServerCmds...)
	res.ServerCmds = list
	response.Success(c, res)
}

func (r *Rustdesk) CmdDelete(c *gin.Context) {
	f := &model.ServerCmd{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	if f.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}

	ex, err := service.AllService.ServerCmdService.Info(f.Id)
	if err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}

	err = service.AllService.ServerCmdService.Delete(ex)
	if err != nil {
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	response.Success(c, nil)
}
func (r *Rustdesk) CmdCreate(c *gin.Context) {
	f := &model.ServerCmd{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	// L'id lo sceglie il database: con quello del corpo un create
	// riscriverebbe o urterebbe una voce esistente.
	f.Id = 0
	err := service.AllService.ServerCmdService.Create(f)
	if err != nil {
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	response.Success(c, nil)
}

// CmdUpdate modifica uno dei comandi del server salvati.
// @Tags Comandi
// @Summary Modifica uno dei comandi del server
// @Description Modifica uno dei comandi del server salvati; solo amministratori.
// @Accept  json
// @Produce  json
// @Param body body model.ServerCmd true "voce dei comandi, con id"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/rustdesk/cmdUpdate [post]
// @Security token
func (r *Rustdesk) CmdUpdate(c *gin.Context) {
	f := &model.ServerCmd{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	if _, err := service.AllService.ServerCmdService.Info(f.Id); err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	err := service.AllService.ServerCmdService.Update(f)
	if err != nil {
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	response.Success(c, nil)
}

func (r *Rustdesk) SendCmd(c *gin.Context) {
	rc := &RustdeskCmd{}
	if err := c.ShouldBindJSON(rc); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	if rc.Cmd == "" {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	if rc.Target == "" {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	if rc.Target != model.ServerCmdTargetIdServer && rc.Target != model.ServerCmdTargetRelayServer {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}

	port := 0
	switch rc.Target {
	case model.ServerCmdTargetIdServer:
		port = global.Config.Admin.IdServerPort - 1
	case model.ServerCmdTargetRelayServer:
		port = global.Config.Admin.RelayServerPort
	}

	res, err := service.AllService.ServerCmdService.SendCmd(port, rc.Cmd, rc.Option)
	if err != nil {
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	response.Success(c, res)
}
