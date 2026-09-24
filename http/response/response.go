package response

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"net/http"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}
type PageData struct {
	Page  int         `json:"page"`
	Total int         `json:"total"`
	List  interface{} `json:"list"`
}

type DataResponse struct {
	Total uint        `json:"total"`
	Data  interface{} `json:"data"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func SendResponse(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		code, message, data,
	})
}

func Success(c *gin.Context, data interface{}) {
	SendResponse(c, 0, "success", data)
}

func Fail(c *gin.Context, code int, message string) {
	SendResponse(c, code, message, nil)
}

func Error(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error: message,
	})
}

type ServerConfigResponse struct {
	IdServer    string `json:"id_server"`
	Key         string `json:"key"`
	RelayServer string `json:"relay_server"`
	ApiServer   string `json:"api_server"`
}

// TranslateMsg traduce il messaggio messageId nella lingua della richiesta.
func TranslateMsg(c *gin.Context, messageId string) string {
	return traduci(c, messageId, nil)
}

// TranslateTempMsg traduce messageId riempiendo il modello con templateData.
func TranslateTempMsg(c *gin.Context, messageId string, templateData map[string]interface{}) string {
	return traduci(c, messageId, templateData)
}

// TranslateParamMsg traduce messageId mettendo params al posto di {{.P0}},
// {{.P1}} e cosi' via.
func TranslateParamMsg(c *gin.Context, messageId string, params ...string) string {
	templateData := make(map[string]interface{})
	for i, v := range params {
		k := fmt.Sprintf("P%d", i)
		templateData[k] = v
	}
	return traduci(c, messageId, templateData)
}

// traduci e' il punto unico delle tre funzioni sopra. Un messaggio che manca
// nella lingua della richiesta ripiega sull'inglese. Un ID che non esiste in
// nessun file, come il testo di un errore di rete passato con err.Error(),
// va nel log a livello warn e non nella risposta, che porta SystemError: i
// dettagli interni non arrivano al client ne' al pannello (REGOLE 8).
func traduci(c *gin.Context, id string, dati map[string]interface{}) string {
	localizer := global.Localizer(c.GetHeader("Accept-Language"))
	msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: id, TemplateData: dati})
	if msg != "" {
		return msg
	}
	global.Logger.Warnf("messaggio %q non tradotto, al client va SystemError: %v", id, err)
	msg, _ = localizer.Localize(&i18n.LocalizeConfig{MessageID: "SystemError"})
	return msg
}
