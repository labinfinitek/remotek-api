package service

import (
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/utils"
)

type UserService struct {
}

// InfoById restituisce l'utente id; ErrNotFound se non c'e'.
func (us *UserService) InfoById(id uint) (*model.User, error) {
	u := &model.User{}
	if err := DB.Where("id = ?", id).First(u).Error; err != nil {
		return nil, fmt.Errorf("utente %d: %w", id, nonTrovato(err))
	}
	return u, nil
}

// InfoByUsername restituisce l'utente di nome un; ErrNotFound se non c'e'.
func (us *UserService) InfoByUsername(un string) (*model.User, error) {
	u := &model.User{}
	if err := DB.Where("username = ?", un).First(u).Error; err != nil {
		return nil, fmt.Errorf("utente per nome: %w", nonTrovato(err))
	}
	return u, nil
}

// InfoByUsernamePassword restituisce l'utente username se password e' la
// sua, con LDAP acceso prima dalla directory; ErrNotFound se l'utente non
// c'e' o la password e' sbagliata. Un errore del database non e' una
// password sbagliata.
func (us *UserService) InfoByUsernamePassword(username, password string) (*model.User, error) {
	if Config.Ldap.Enable {
		u, err := AllService.LdapService.Authenticate(username, password)
		if err == nil {
			return u, nil
		}
		Logger.Errorf("LDAP authentication failed, %v", err)
		Logger.Warn("Fallback to local database")
	}
	u, err := us.InfoByUsername(username)
	if err != nil {
		return nil, err
	}
	if ok, err := utils.VerifyPassword(u.Password, password); err != nil || !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

// InfoByAccessToken restituisce l'utente del token di sessione token e il
// token; ErrNotFound se il token non c'e' o e' scaduto, o se il suo utente
// non c'e' piu'.
func (us *UserService) InfoByAccessToken(token string) (*model.User, *model.UserToken, error) {
	ut := &model.UserToken{}
	if err := DB.Where("token = ?", token).First(ut).Error; err != nil {
		return nil, nil, fmt.Errorf("token di sessione: %w", nonTrovato(err))
	}
	if ut.ExpiredAt < time.Now().Unix() {
		return nil, nil, fmt.Errorf("token di sessione %d scaduto: %w", ut.Id, ErrNotFound)
	}
	u := &model.User{}
	if err := DB.Where("id = ?", ut.UserId).First(u).Error; err != nil {
		return nil, nil, fmt.Errorf("utente %d del token di sessione: %w", ut.UserId, nonTrovato(err))
	}
	return u, ut, nil
}

// fonteCasuale e' la fonte dei token di sessione; i test la sostituiscono
// per vedere che il token e' fatto solo dei byte letti da qui.
var fonteCasuale io.Reader = crand.Reader

// GenerateToken restituisce il token di sessione: un JWT se jwt.key e'
// impostata, altrimenti 16 byte da crypto/rand in esadecimale (ADR-0008),
// 32 caratteri come il vecchio md5(username + ora), che si poteva indovinare.
func (us *UserService) GenerateToken(u *model.User) string {
	if len(Jwt.Key) > 0 {
		return Jwt.GenerateToken(u.Id)
	}
	token, err := tokenCasuale(fonteCasuale)
	if err != nil {
		// Con Go 1.26 un guasto della fonte di sistema ferma il processo
		// dentro crypto/rand e qui non si arriva. Se ci si arriva, si
		// interrompe la richiesta prima di salvare il token: meglio un
		// login fallito che un token prevedibile.
		panic(err)
	}
	return token
}

// tokenCasuale legge 16 byte da r e li restituisce in esadecimale. Se r
// fallisce o ne da' meno di 16 restituisce l'errore e nessun token.
func tokenCasuale(r io.Reader) (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", fmt.Errorf("token di sessione da crypto/rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Login crea il token di sessione dell'utente u e registra l'accesso llog,
// in una transazione che su errore o panic si annulla: chi entra non riceve
// un token che il database non ha. Poi lega all'utente il dispositivo di
// llog, se c'e'; se non ci riesce il login vale lo stesso e l'errore va nel
// log.
func (us *UserService) Login(u *model.User, llog *model.LoginLog) (*model.UserToken, error) {
	token := us.GenerateToken(u)
	ut := &model.UserToken{
		UserId:     u.Id,
		Token:      token,
		DeviceUuid: llog.Uuid,
		DeviceId:   llog.DeviceId,
		ExpiredAt:  us.UserTokenExpireTimestamp(),
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ut).Error; err != nil {
			return fmt.Errorf("token di sessione: %w", err)
		}
		llog.UserTokenId = ut.UserId
		if err := tx.Create(llog).Error; err != nil {
			return fmt.Errorf("registro degli accessi: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("login dell'utente %d: %w", u.Id, err)
	}
	if llog.Uuid != "" {
		if err := AllService.PeerService.UuidBindUserId(llog.Uuid, u.Id); err != nil {
			Logger.Warnf("login dell'utente %d: il dispositivo non si lega all'utente: %v", u.Id, err)
		}
	}
	return ut, nil
}

// CurUser 获取当前用户
func (us *UserService) CurUser(c *gin.Context) *model.User {
	user, _ := c.Get("curUser")
	u, ok := user.(*model.User)
	if !ok {
		return nil
	}
	return u
}

func (us *UserService) List(page, pageSize uint, where func(tx *gorm.DB)) (res *model.UserList, err error) {
	res = &model.UserList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.User{})
	if where != nil {
		where(tx)
	}
	if err = tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio degli utenti: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err = tx.Find(&res.Users).Error; err != nil {
		return nil, fmt.Errorf("utenti: %w", err)
	}
	return res, nil
}

func (us *UserService) ListByIds(ids []uint) (res []*model.User, err error) {
	if err = DB.Where("id in ?", ids).Find(&res).Error; err != nil {
		return nil, fmt.Errorf("utenti per id: %w", err)
	}
	return res, nil
}

// ListByGroupId restituisce la pagina page di pageSize utenti del gruppo
// groupId.
func (us *UserService) ListByGroupId(groupId, page, pageSize uint) (*model.UserList, error) {
	return us.List(page, pageSize, func(tx *gorm.DB) {
		tx.Where("group_id = ?", groupId)
	})
}

// ListIdAndNameByGroupId restituisce id e nome degli utenti del gruppo
// groupId.
func (us *UserService) ListIdAndNameByGroupId(groupId uint) (res []*model.User, err error) {
	if err = DB.Model(&model.User{}).Where("group_id = ?", groupId).Select("id, username").Find(&res).Error; err != nil {
		return nil, fmt.Errorf("utenti del gruppo %d: %w", groupId, err)
	}
	return res, nil
}

// CheckUserEnable 判断用户是否禁用
func (us *UserService) CheckUserEnable(u *model.User) bool {
	return u.Status == model.COMMON_STATUS_ENABLE
}

// Create 创建
func (us *UserService) Create(u *model.User) error {
	// The initial username should be formatted, and the username should be unique
	esiste, err := us.IsUsernameExists(u.Username)
	if err != nil {
		return diSistema(err)
	}
	if esiste {
		return errors.New("UsernameExists")
	}
	u.Username = us.formatUsername(u.Username)
	u.Password, err = utils.EncryptPassword(u.Password)
	if err != nil {
		return err
	}
	res := DB.Create(u).Error
	return res
}

// Logout cancella il token di sessione token dell'utente u e scollega
// dall'utente il dispositivo del token, se ce n'e' uno.
func (us *UserService) Logout(u *model.User, token string) error {
	ut := &model.UserToken{}
	err := DB.Where("user_id = ? and token = ?", u.Id, token).First(ut).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lettura del token: %w", err)
	}
	if err := DB.Delete(ut).Error; err != nil {
		return fmt.Errorf("cancellazione del token: %w", err)
	}
	if ut.DeviceUuid == "" {
		return nil
	}
	if err := AllService.PeerService.UuidUnbindUserId(ut.DeviceUuid, u.Id); err != nil {
		return fmt.Errorf("dispositivo del token: %w", err)
	}
	return nil
}

// Delete cancella l'utente con le sue associazioni ai provider e le sue
// voci, collezioni e regole della rubrica, in una transazione che su errore
// o panic si annulla; poi scollega i suoi dispositivi. Un amministratore
// non si cancella se e' l'ultimo o se gli amministratori non si contano.
func (us *UserService) Delete(u *model.User) error {
	if us.IsAdmin(u) {
		userCount, err := us.getAdminUserCount()
		if err != nil {
			return diSistema(err)
		}
		if userCount <= 1 {
			return errors.New("the last admin user cannot be deleted")
		}
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(u).Error; err != nil {
			return fmt.Errorf("utente %d: %w", u.Id, err)
		}
		for _, m := range []any{&model.UserThird{}, &model.AddressBook{}, &model.AddressBookCollection{}, &model.AddressBookCollectionRule{}} {
			if err := tx.Where("user_id = ?", u.Id).Delete(m).Error; err != nil {
				return fmt.Errorf("%T dell'utente %d: %w", m, u.Id, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 删除关联的peer
	if err := AllService.PeerService.EraseUserId(u.Id); err != nil {
		Logger.Warn("User deleted successfully, but failed to unlink peer.")
		return nil
	}
	return nil
}

// Update salva i campi non vuoti dell'utente u. L'ultimo amministratore non
// si disabilita e non perde il ruolo; se l'utente o il numero degli
// amministratori non si leggono, non salva niente.
func (us *UserService) Update(u *model.User) error {
	if err := us.controllaUltimoAdmin(u); err != nil {
		return err
	}
	return DB.Model(u).Updates(u).Error
}

// UpdateDalPannello salva, con gli stessi controlli di Update, i campi del
// modulo del pannello (admin.UserForm), anche vuoti: email, nickname, avatar
// e nota svuotati restano vuoti. is_admin assente non cambia il ruolo; la
// password non e' nel modulo e resta quella di prima.
func (us *UserService) UpdateDalPannello(u *model.User) error {
	if err := us.controllaUltimoAdmin(u); err != nil {
		return err
	}
	campi := []string{"username", "email", "nickname", "avatar", "group_id", "status", "remark"}
	if u.IsAdmin != nil {
		campi = append(campi, "is_admin")
	}
	if err := DB.Model(u).Select(campi).Updates(u).Error; err != nil {
		return fmt.Errorf("utente %d: %w", u.Id, err)
	}
	return nil
}

// controllaUltimoAdmin restituisce un errore se u disabiliterebbe o
// toglierebbe il ruolo all'ultimo amministratore, o se l'utente o il numero
// degli amministratori non si leggono.
func (us *UserService) controllaUltimoAdmin(u *model.User) error {
	currentUser, err := us.InfoById(u.Id)
	if err != nil {
		return diSistema(err)
	}
	// 如果当前用户是管理员并且 IsAdmin 不为空，进行检查
	if us.IsAdmin(currentUser) {
		adminCount, err := us.getAdminUserCount()
		if err != nil {
			return diSistema(err)
		}
		// 如果这是唯一的管理员，确保不能禁用或取消管理员权限
		if adminCount <= 1 && (!us.IsAdmin(u) || u.Status == model.COMMON_STATUS_DISABLED) {
			return errors.New("the last admin user cannot be disabled or demoted")
		}
	}
	return nil
}

// FlushToken 清空token
func (us *UserService) FlushToken(u *model.User) error {
	return DB.Where("user_id = ?", u.Id).Delete(&model.UserToken{}).Error
}

// FlushTokenByUuid 清空token
func (us *UserService) FlushTokenByUuid(uuid string) error {
	return DB.Where("device_uuid = ?", uuid).Delete(&model.UserToken{}).Error
}

// UpdatePassword 更新密码
func (us *UserService) UpdatePassword(u *model.User, password string) error {
	var err error
	u.Password, err = utils.EncryptPassword(password)
	if err != nil {
		return err
	}
	err = DB.Model(u).Update("password", u.Password).Error
	if err != nil {
		return err
	}
	err = us.FlushToken(u)
	return err
}

// IsAdmin 是否管理员
func (us *UserService) IsAdmin(u *model.User) bool {
	return u != nil && *u.IsAdmin
}

// RouteNames restituisce i nomi delle rotte del pannello che l'utente u vede.
func (us *UserService) RouteNames(u *model.User) []string {
	if us.IsAdmin(u) {
		return model.AdminRouteNames
	}
	return model.UserRouteNames
}

// InfoByOauthId restituisce l'utente dell'associazione all'account openId del
// provider op; ErrNotFound se non c'e' l'associazione o il suo utente.
func (us *UserService) InfoByOauthId(op string, openId string) (*model.User, error) {
	ut, err := AllService.OauthService.UserThirdInfo(op, openId)
	if err != nil {
		return nil, err
	}
	return us.InfoById(ut.UserId)
}

// RegisterByOauth restituisce l'utente locale di oauthUser, l'utente del
// provider op: quello della sua associazione al provider; se non c'e',
// quello con la stessa email, a cui aggiunge l'associazione; altrimenti uno
// nuovo, creato con l'associazione in una transazione. Un errore del
// database ferma tutto: una lettura che fallisce non vale "non trovato", che
// farebbe un utente doppio.
func (us *UserService) RegisterByOauth(oauthUser *model.OauthUser, op string) (*model.User, error) {
	Lock.Lock("registerByOauth")
	defer Lock.UnLock("registerByOauth")
	ut := &model.UserThird{}
	switch err := DB.Where("open_id = ? and op = ?", oauthUser.OpenId, op).First(ut).Error; {
	case err == nil:
		user := &model.User{}
		if err := DB.Where("id = ?", ut.UserId).First(user).Error; err != nil {
			return nil, fmt.Errorf("utente %d dell'associazione al provider: %w", ut.UserId, err)
		}
		return user, nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, fmt.Errorf("associazione al provider: %w", err)
	}
	oauthType, err := AllService.OauthService.GetTypeByOp(op)
	if err != nil {
		return nil, err
	}
	// check if this email has been registered
	email := oauthUser.Email
	// only email is not empty
	if email != "" {
		email = strings.ToLower(email)
		// update email to oauthUser, in case it contain upper case
		oauthUser.Email = email
		// call this, if find user by email, it will update the email to local database
		user, ldapErr := AllService.LdapService.GetUserInfoByEmailLocal(email)
		// If we enable ldap, and the error is not ErrLdapUserNotFound, return the error because we could not sure if the user is not found in ldap
		if !errors.Is(ldapErr, ErrLdapNotEnabled) && !errors.Is(ldapErr, ErrLdapUserNotFound) && ldapErr != nil {
			return user, ldapErr
		}
		if user.Id == 0 {
			// this means the user is not found in ldap, maybe ldao is not enabled
			user = &model.User{}
			switch err := DB.Where("email = ?", email).First(user).Error; {
			case errors.Is(err, gorm.ErrRecordNotFound):
				user = nil
			case err != nil:
				return nil, fmt.Errorf("utente con l'email del provider: %w", err)
			}
		}
		if user != nil {
			ut.FromOauthUser(user.Id, oauthUser, oauthType, op)
			if err := DB.Create(ut).Error; err != nil {
				return nil, errors.Join(errors.New("OauthRegisterFailed"), fmt.Errorf("associazione al provider dell'utente %d con la stessa email: %w", user.Id, err))
			}
			return user, nil
		}
	}

	ut = &model.UserThird{}
	ut.FromOauthUser(0, oauthUser, oauthType, op)
	// The initial username should be formatted
	username := us.formatUsername(oauthUser.Username)
	// Il nome libero si cerca prima della transazione: la ricerca passa da DB
	// e, con LDAP acceso, dalla rete, e con una connessione sola DB
	// aspetterebbe per sempre la connessione della transazione.
	usernameUnique, err := us.GenerateUsernameByOauth(username)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Username: usernameUnique,
		GroupId:  1,
	}
	oauthUser.ToUser(user, false)
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return fmt.Errorf("utente: %w", err)
		}
		ut.UserId = user.Id
		if err := tx.Create(ut).Error; err != nil {
			return fmt.Errorf("associazione al provider: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Join(errors.New("OauthRegisterFailed"), err)
	}
	return user, nil
}

// GenerateUsernameByOauth restituisce name o, se e' preso, name con cifre in
// coda finche' non e' libero. Se non si sa se un nome e' preso si ferma, con
// l'errore.
func (us *UserService) GenerateUsernameByOauth(name string) (string, error) {
	for {
		preso, err := us.IsUsernameExists(name)
		if err != nil || !preso {
			return name, err
		}
		name += strconv.Itoa(rand.Intn(10)) //nolint:gosec // G404: una cifra in coda a un nome gia' preso, non un segreto
	}
}

// UserThirdsByUserId restituisce le associazioni ai provider dell'utente
// userId.
func (us *UserService) UserThirdsByUserId(userId uint) (res []*model.UserThird, err error) {
	if err = DB.Where("user_id = ?", userId).Find(&res).Error; err != nil {
		return nil, fmt.Errorf("associazioni ai provider dell'utente %d: %w", userId, err)
	}
	return res, nil
}

// UserThirdInfo restituisce l'associazione dell'utente userId al provider
// op; ErrNotFound se non c'e'.
func (us *UserService) UserThirdInfo(userId uint, op string) (*model.UserThird, error) {
	ut := &model.UserThird{}
	if err := DB.Where("user_id = ? and op = ?", userId, op).First(ut).Error; err != nil {
		return nil, fmt.Errorf("associazione dell'utente %d al provider: %w", userId, nonTrovato(err))
	}
	return ut, nil
}

// FindLatestUserIdFromLoginLogByUuid restituisce l'utente dell'ultimo login
// del dispositivo uuid con id deviceId; ErrNotFound se non ce n'e'.
func (us *UserService) FindLatestUserIdFromLoginLogByUuid(uuid string, deviceId string) (uint, error) {
	llog := &model.LoginLog{}
	if err := DB.Where("uuid = ? and device_id = ?", uuid, deviceId).Order("id desc").First(llog).Error; err != nil {
		return 0, fmt.Errorf("ultimo login del dispositivo: %w", nonTrovato(err))
	}
	return llog.UserId, nil
}

// IsPasswordEmptyById 根据用户id判断密码是否为空，主要用于第三方登录的自动注册
func (us *UserService) IsPasswordEmptyById(id uint) bool {
	u := &model.User{}
	if DB.Where("id = ?", id).First(u).Error != nil {
		return false
	}
	return u.Password == ""
}

// IsPasswordEmptyByUsername 根据用户id判断密码是否为空，主要用于第三方登录的自动注册
func (us *UserService) IsPasswordEmptyByUsername(username string) bool {
	u := &model.User{}
	if DB.Where("username = ?", username).First(u).Error != nil {
		return false
	}
	return u.Password == ""
}

// IsPasswordEmptyByUser 判断密码是否为空，主要用于第三方登录的自动注册
func (us *UserService) IsPasswordEmptyByUser(u *model.User) bool {
	return us.IsPasswordEmptyById(u.Id)
}

// Register 注册, 如果用户名已存在则返回nil
func (us *UserService) Register(username string, email string, password string, status model.StatusCode) *model.User {
	u := &model.User{
		Username: username,
		Email:    email,
		Password: password,
		GroupId:  1,
		Status:   status,
	}
	err := us.Create(u)
	if err != nil {
		return nil
	}
	return u
}

func (us *UserService) TokenList(page uint, size uint, f func(tx *gorm.DB)) (*model.UserTokenList, error) {
	res := &model.UserTokenList{}
	res.Page = int64(page)
	res.PageSize = int64(size)
	tx := DB.Model(&model.UserToken{})
	if f != nil {
		f(tx)
	}
	if err := tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio dei token di sessione: %w", err)
	}
	tx.Scopes(Paginate(page, size))
	if err := tx.Find(&res.UserTokens).Error; err != nil {
		return nil, fmt.Errorf("token di sessione: %w", err)
	}
	return res, nil
}

// TokenInfoById restituisce il token di sessione id; ErrNotFound se non
// c'e'.
func (us *UserService) TokenInfoById(id uint) (*model.UserToken, error) {
	ut := &model.UserToken{}
	if err := DB.Where("id = ?", id).First(ut).Error; err != nil {
		return nil, fmt.Errorf("token di sessione %d: %w", id, nonTrovato(err))
	}
	return ut, nil
}

func (us *UserService) DeleteToken(l *model.UserToken) error {
	return DB.Delete(l).Error
}

// Helper functions, used for formatting username
func (us *UserService) formatUsername(username string) string {
	username = strings.ReplaceAll(username, " ", "")
	username = strings.ToLower(username)
	return username
}

// getAdminUserCount restituisce quanti amministratori ci sono.
func (us *UserService) getAdminUserCount() (int64, error) {
	var count int64
	if err := DB.Model(&model.User{}).Where("is_admin = ?", true).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("conteggio degli amministratori: %w", err)
	}
	return count, nil
}

// UserTokenExpireTimestamp 生成用户token过期时间
func (us *UserService) UserTokenExpireTimestamp() int64 {
	exp := Config.App.TokenExpire
	if exp == 0 {
		// 默认七天
		exp = 604800
	}
	return time.Now().Add(exp).Unix()
}

// RefreshAccessToken porta la scadenza del token ut a quella di un token
// nuovo.
func (us *UserService) RefreshAccessToken(ut *model.UserToken) error {
	ut.ExpiredAt = us.UserTokenExpireTimestamp()
	if err := DB.Model(ut).Update("expired_at", ut.ExpiredAt).Error; err != nil {
		return fmt.Errorf("scadenza del token %d: %w", ut.Id, err)
	}
	return nil
}

// AutoRefreshAccessToken rinnova il token ut se gli manca meno di un terzo
// di app.token-expire.
func (us *UserService) AutoRefreshAccessToken(ut *model.UserToken) error {
	if ut.ExpiredAt-time.Now().Unix() < Config.App.TokenExpire.Milliseconds()/3000 {
		return us.RefreshAccessToken(ut)
	}
	return nil
}

func (us *UserService) BatchDeleteUserToken(ids []uint) error {
	return DB.Where("id in ?", ids).Delete(&model.UserToken{}).Error
}

func (us *UserService) VerifyJWT(token string) (uint, error) {
	return Jwt.ParseToken(token)
}

// IsUsernameExists dice se il nome username e' preso, nel database o, con
// LDAP acceso, nella directory.
func (us *UserService) IsUsernameExists(username string) (bool, error) {
	locale, err := us.IsUsernameExistsLocal(username)
	if err != nil {
		return false, err
	}
	return locale || AllService.LdapService.IsUsernameExists(username), nil
}

// IsUsernameExistsLocal dice se nel database c'e' un utente di nome username.
func (us *UserService) IsUsernameExistsLocal(username string) (bool, error) {
	_, err := us.InfoByUsername(username)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (us *UserService) IsEmailExistsLdap(email string) bool {
	return AllService.LdapService.IsEmailExists(email)
}
