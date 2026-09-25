package response

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/lejianwen/rustdesk-api/v2/global"
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

// ErrorErr risponde come Error, 400 con {"error": ...}, senza il testo di
// err, che e' interno e va solo nel log (REGOLE 8). Il messaggio e' quello
// tradotto del primo ID nella catena di err, come errors.New("UsernameExists")
// avvolto con %w o ErrLdapConnectFailed unito al dettaglio con errors.Join;
// se la catena non ne ha, quello di id, il messaggio del punto.
func ErrorErr(c *gin.Context, id string, err error) {
	Error(c, messaggioPer(c, id, err))
}

// FailErr risponde come Fail, col codice code, con il messaggio che ErrorErr
// sceglie per id ed err; anche qui il testo di err va solo nel log.
func FailErr(c *gin.Context, code int, id string, err error) {
	Fail(c, code, messaggioPer(c, id, err))
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

// messaggioPer scrive err nel log a livello warn e restituisce il messaggio
// tradotto che va al client al suo posto. Nel log vanno il metodo, la rotta
// del router (/api/ab/peer/add/:guid, non il percorso coi valori della
// richiesta) e l'errore con %q, che tiene su una riga gli a capo di
// errors.Join.
func messaggioPer(c *gin.Context, id string, err error) string {
	localizer := global.Localizer(c.GetHeader("Accept-Language"))
	if trovato := idNellaCatena(localizer, err); trovato != "" {
		id = trovato
	}
	global.Logger.Warnf("%s %s: al client va %s, errore %q", c.Request.Method, c.FullPath(), id, err)
	return traduci(c, id, nil)
}

// idNellaCatena restituisce il testo del primo errore della catena di err,
// visitata come fa errors.Is (anche dentro errors.Join), che e' l'ID di un
// messaggio; "" se non ce n'e'. Non passa da traduci, che scriverebbe un warn
// per ogni testo che non e' un ID.
func idNellaCatena(localizer *i18n.Localizer, err error) string {
	for ; err != nil; err = errors.Unwrap(err) {
		if msg, _ := localizer.Localize(&i18n.LocalizeConfig{MessageID: err.Error()}); msg != "" {
			return err.Error()
		}
		if unito, ok := err.(interface{ Unwrap() []error }); ok {
			for _, figlio := range unito.Unwrap() {
				if id := idNellaCatena(localizer, figlio); id != "" {
					return id
				}
			}
			return ""
		}
	}
	return ""
}
