package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/request/admin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

type Audit struct {
}

// ConnList 列表
// @Tags 链接日志
// @Summary 链接日志列表
// @Description 链接日志列表
// @Accept  json
// @Produce  json
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Param peer_id query int false "目标设备"
// @Param from_peer query int false "来源设备"
// @Success 200 {object} response.Response{data=model.AuditConnList}
// @Failure 500 {object} response.Response
// @Router /admin/audit_conn/list [get]
// @Security token
func (a *Audit) ConnList(c *gin.Context) {
	query := &admin.AuditQuery{}
	if err := c.ShouldBindQuery(query); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	res, err := service.AllService.AuditService.AuditConnList(query.Page, query.PageSize, func(tx *gorm.DB) {
		if query.PeerId != "" {
			tx.Where("peer_id like ?", "%"+query.PeerId+"%")
		}
		if query.FromPeer != "" {
			tx.Where("from_peer like ?", "%"+query.FromPeer+"%")
		}
		tx.Order("id desc")
	})
	if err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	response.Success(c, res)
}

// ConnDelete 删除
// @Tags 链接日志
// @Summary 链接日志删除
// @Description 链接日志删除
// @Accept  json
// @Produce  json
// @Param body body model.AuditConn true "链接日志信息"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/audit_conn/delete [post]
// @Security token
func (a *Audit) ConnDelete(c *gin.Context) {
	f := &model.AuditConn{}
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
	l, err := service.AllService.AuditService.ConnInfoById(f.Id)
	if err == nil {
		err := service.AllService.AuditService.DeleteAuditConn(l)
		if err == nil {
			response.Success(c, nil)
			return
		}
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	response.FailErr(c, 101, "SystemError", err)
}

// BatchConnDelete 删除
// @Tags 链接日志
// @Summary 链接日志批量删除
// @Description 链接日志批量删除
// @Accept  json
// @Produce  json
// @Param body body admin.AuditConnLogIds true "链接日志"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/audit_conn/batchDelete [post]
// @Security token
func (a *Audit) BatchConnDelete(c *gin.Context) {
	f := &admin.AuditConnLogIds{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	if len(f.Ids) == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}

	err := service.AllService.AuditService.BatchDeleteAuditConn(f.Ids)
	if err == nil {
		response.Success(c, nil)
		return
	}
	response.FailErr(c, 101, "OperationFailed", err)
}

// FileList 列表
// @Tags 文件日志
// @Summary 文件日志列表
// @Description 文件日志列表
// @Accept  json
// @Produce  json
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Param peer_id query int false "目标设备"
// @Param from_peer query int false "来源设备"
// @Success 200 {object} response.Response{data=model.AuditFileList}
// @Failure 500 {object} response.Response
// @Router /admin/audit_file/list [get]
// @Security token
func (a *Audit) FileList(c *gin.Context) {
	query := &admin.AuditQuery{}
	if err := c.ShouldBindQuery(query); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	res, err := service.AllService.AuditService.AuditFileList(query.Page, query.PageSize, func(tx *gorm.DB) {
		if query.PeerId != "" {
			tx.Where("peer_id like ?", "%"+query.PeerId+"%")
		}
		if query.FromPeer != "" {
			tx.Where("from_peer like ?", "%"+query.FromPeer+"%")
		}
		tx.Order("id desc")
	})
	if err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	response.Success(c, res)
}

// FileDelete 删除
// @Tags 文件日志
// @Summary 文件日志删除
// @Description 文件日志删除
// @Accept  json
// @Produce  json
// @Param body body model.AuditFile true "文件日志信息"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/audit_file/delete [post]
// @Security token
func (a *Audit) FileDelete(c *gin.Context) {
	f := &model.AuditFile{}
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
	l, err := service.AllService.AuditService.FileInfoById(f.Id)
	if err == nil {
		err := service.AllService.AuditService.DeleteAuditFile(l)
		if err == nil {
			response.Success(c, nil)
			return
		}
		response.FailErr(c, 101, "OperationFailed", err)
		return
	}
	response.FailErr(c, 101, "SystemError", err)
}

// BatchFileDelete 删除
// @Tags 文件日志
// @Summary 文件日志批量删除
// @Description 文件日志批量删除
// @Accept  json
// @Produce  json
// @Param body body admin.AuditFileLogIds true "文件日志"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/audit_file/batchDelete [post]
// @Security token
func (a *Audit) BatchFileDelete(c *gin.Context) {
	f := &admin.AuditFileLogIds{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	if len(f.Ids) == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}

	err := service.AllService.AuditService.BatchDeleteAuditFile(f.Ids)
	if err == nil {
		response.Success(c, nil)
		return
	}
	response.FailErr(c, 101, "OperationFailed", err)
}

// TerminalList restituisce una pagina dei blocchi della trascrizione della
// connessione audit_conn_id, in ordine di seq, con data in base64.
// @Tags 链接日志
// @Summary blocchi della trascrizione del terminale
// @Produce  json
// @Param audit_conn_id query int true "id della connessione nel registro"
// @Param page query int false "pagina"
// @Param page_size query int false "blocchi per pagina"
// @Success 200 {object} response.Response{data=model.AuditTerminalList}
// @Failure 500 {object} response.Response
// @Router /admin/audit_conn/terminal/list [get]
// @Security token
func (a *Audit) TerminalList(c *gin.Context) {
	query := &admin.AuditTerminalQuery{}
	if err := c.ShouldBindQuery(query); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	res, err := service.AllService.AuditService.BlocchiTerminale(query.AuditConnId, query.Page, query.PageSize)
	if err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	response.Success(c, res)
}

// TerminalVerify ricalcola dal database la catena degli hash della
// trascrizione della connessione audit_conn_id.
// @Tags 链接日志
// @Summary verifica della trascrizione del terminale
// @Produce  json
// @Param audit_conn_id query int true "id della connessione nel registro"
// @Success 200 {object} response.Response{data=model.AuditTerminalVerifica}
// @Failure 500 {object} response.Response
// @Router /admin/audit_conn/terminal/verify [get]
// @Security token
func (a *Audit) TerminalVerify(c *gin.Context) {
	query := &admin.AuditTerminalQuery{}
	if err := c.ShouldBindQuery(query); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	res, err := service.AllService.AuditService.VerificaTerminale(query.AuditConnId)
	if err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	response.Success(c, res)
}

// TerminalExport scrive come file JSON da scaricare la trascrizione della
// connessione audit_conn_id: la riga del registro, i blocchi in ordine di
// seq e la verifica calcolata sugli stessi blocchi. I blocchi si leggono a
// pagine (ScorriTerminale) e vanno nella risposta man mano, senza tenere il
// database mentre si scrive a chi scarica. Un id che non c'e'
// o che non e' un terminale risponde l'errore del pannello, senza file.
// @Tags 链接日志
// @Summary esportazione della trascrizione del terminale
// @Produce  json
// @Param audit_conn_id query int true "id della connessione nel registro"
// @Success 200 {object} model.AuditTerminalEsportazione
// @Failure 500 {object} response.Response
// @Router /admin/audit_conn/terminal/export [get]
// @Security token
func (a *Audit) TerminalExport(c *gin.Context) {
	query := &admin.AuditTerminalQuery{}
	if err := c.ShouldBindQuery(query); err != nil {
		response.FailErr(c, 101, "ParamsError", err)
		return
	}
	as := service.AllService.AuditService
	conn, err := as.ConnInfoById(query.AuditConnId)
	if err == nil && conn.Type != model.AuditConnTerminale {
		err = fmt.Errorf("connessione %d di tipo %d: %w", conn.Id, conn.Type, service.ErrNotFound)
	}
	if err != nil {
		id := "SystemError"
		if errors.Is(err, service.ErrNotFound) {
			id = "ItemNotFound"
		}
		response.FailErr(c, 101, id, err)
		return
	}
	riga, err := json.Marshal(conn)
	if err != nil {
		response.FailErr(c, 101, "SystemError", err)
		return
	}
	// L'intestazione e la riga si scrivono col primo blocco o a fine
	// lettura: un errore della query arriva al pannello come gli altri.
	iniziato := false
	inizia := func() error {
		iniziato = true
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="trascrizione-%d.json"`, conn.Id))
		c.Status(http.StatusOK)
		_, err := fmt.Fprintf(c.Writer, `{"connessione":%s,"blocchi":[`, riga)
		return err
	}
	verifica, err := as.ScorriTerminale(conn.Id, func(b *model.AuditTerminal) error {
		var err error
		if !iniziato {
			err = inizia()
		} else {
			_, err = c.Writer.WriteString(",")
		}
		if err != nil {
			return err
		}
		pezzo, err := json.Marshal(&model.AuditTerminalEsportato{Seq: b.Seq, Dir: b.Dir, Data: b.Data, Hash: b.Hash, Fine: b.Fine, CreatedAt: b.CreatedAt})
		if err == nil {
			_, err = c.Writer.Write(pezzo)
		}
		return err
	})
	if err == nil && !iniziato {
		err = inizia()
	}
	var fine []byte
	if err == nil {
		fine, err = json.Marshal(verifica)
	}
	if err == nil {
		_, err = fmt.Fprintf(c.Writer, `],"verifica":%s}`, fine)
	}
	if err != nil {
		if !iniziato {
			response.FailErr(c, 101, "SystemError", err)
			return
		}
		global.Logger.Per(c.Request.Context()).Errorf("%s %s: trascrizione della connessione %d interrotta: %v", c.Request.Method, c.FullPath(), conn.Id, err)
		interrompi(c)
		return
	}
	var amministratore uint
	if u := service.AllService.UserService.CurUser(c); u != nil {
		amministratore = u.Id
	}
	global.Logger.Per(c.Request.Context()).Infof("%s %s: l'amministratore %d ha esportato la trascrizione della connessione %d", c.Request.Method, c.FullPath(), amministratore, conn.Id)
}

// interrompi chiude la connessione di una risposta gia' cominciata, cosi'
// che chi scarica veda un errore e non un file troncato che sembra intero.
func interrompi(c *gin.Context) {
	if conn, _, err := c.Writer.Hijack(); err == nil {
		_ = conn.Close()
	}
}
