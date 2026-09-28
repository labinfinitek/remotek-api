package service

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/lib/lock"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

type Service struct {
	// AdminService     *AdminService
	// AdminRoleService *AdminRoleService
	*UserService
	*AddressBookService
	*TagService
	*PeerService
	*GroupService
	*OauthService
	*LoginLogService
	*AuditService
	*ServerCmdService
	*LdapService
	*AppService
}

type Dependencies struct {
	Config *config.Config
	DB     *gorm.DB
	Logger *log.Logger
	Jwt    *jwt.Jwt
	Lock   *lock.Locker
}

var Config *config.Config
var DB *gorm.DB
var Logger *log.Logger
var Jwt *jwt.Jwt
var Lock lock.Locker

var AllService *Service

func New(c *config.Config, g *gorm.DB, l *log.Logger, j *jwt.Jwt, lo lock.Locker) *Service {
	Config = c
	DB = g
	Logger = l
	Jwt = j
	Lock = lo
	AllService = new(Service)
	return AllService
}

// ErrNotFound dice che una lettura non ha trovato la riga cercata. Il testo e'
// l'ID del messaggio ItemNotFound: ErrorErr e FailErr lo trovano nella catena
// dell'errore, mentre un errore del database prende il messaggio del punto.
var ErrNotFound = errors.New("ItemNotFound")

// nonTrovato restituisce ErrNotFound se err e' gorm.ErrRecordNotFound,
// altrimenti err.
func nonTrovato(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// diSistema aggiunge l'ID SystemError alla catena di err, se non e'
// ErrNotFound: dove il messaggio del punto e' un altro, come OperationFailed,
// FailErr dice "Errore di sistema." per una lettura che non riesce e
// "Elemento non trovato." per una riga che non c'e'.
func diSistema(err error) error {
	if errors.Is(err, ErrNotFound) {
		return err
	}
	return errors.Join(errors.New("SystemError"), err)
}

func Paginate(page, pageSize uint) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if page == 0 {
			page = 1
		}
		if pageSize == 0 {
			pageSize = 10
		}
		offset := (page - 1) * pageSize
		return db.Offset(int(offset)).Limit(int(pageSize))
	}
}

func CommonEnable() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ?", model.COMMON_STATUS_ENABLE)
	}
}
