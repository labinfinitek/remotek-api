package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/utils"
)

type OauthService struct {
}

// OidcEndpoint is the response of .well-known/openid-configuration.
type OidcEndpoint struct {
	Issuer   string `json:"issuer"`
	AuthURL  string `json:"authorization_endpoint"`
	TokenURL string `json:"token_endpoint"`
	UserInfo string `json:"userinfo_endpoint"`
}

type OauthCacheItem struct {
	UserId     uint   `json:"user_id"`
	Id         string `json:"id"` // rustdesk的设备ID
	Op         string `json:"op"`
	Action     string `json:"action"`
	Uuid       string `json:"uuid"`
	DeviceName string `json:"device_name"`
	DeviceOs   string `json:"device_os"`
	DeviceType string `json:"device_type"`
	OpenId     string `json:"open_id"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Verifier   string `json:"verifier"` // used for oauth pkce
	Nonce      string `json:"nonce"`
}

func (oci *OauthCacheItem) ToOauthUser() *model.OauthUser {
	return &model.OauthUser{
		OpenId:   oci.OpenId,
		Username: oci.Username,
		Name:     oci.Name,
		Email:    oci.Email,
	}
}

var OauthCache = &sync.Map{}

const (
	OauthActionTypeLogin = "login"
	OauthActionTypeBind  = "bind"
)

func (oci *OauthCacheItem) UpdateFromOauthUser(oauthUser *model.OauthUser) {
	oci.OpenId = oauthUser.OpenId
	oci.Username = oauthUser.Username
	oci.Name = oauthUser.Name
	oci.Email = oauthUser.Email
}

func (os *OauthService) GetOauthCache(key string) *OauthCacheItem {
	v, ok := OauthCache.Load(key)
	if !ok {
		return nil
	}
	return v.(*OauthCacheItem)
}

func (os *OauthService) SetOauthCache(key string, item *OauthCacheItem, expire uint) {
	OauthCache.Store(key, item)
	if expire > 0 {
		time.AfterFunc(time.Duration(expire)*time.Second, func() {
			os.DeleteOauthCache(key)
		})
	}
}

func (os *OauthService) DeleteOauthCache(key string) {
	OauthCache.Delete(key)
}

func (os *OauthService) BeginAuth(op string) (state, verifier, nonce, url string, err error) {
	state = utils.RandomString(10) + strconv.FormatInt(time.Now().Unix(), 10)
	verifier = ""
	nonce = ""
	if op == model.OauthTypeWebauth {
		// Con app.web-sso spento webauth non esiste: stessa risposta di un op
		// che non c'e', non solo la voce tolta da /api/login-options.
		if !Config.App.WebSso {
			return state, verifier, nonce, "", errors.New("ConfigNotFound")
		}
		url = Config.Rustdesk.ApiServer + "/_admin/#/oauth/" + state
		// url = "http://localhost:8888/_admin/#/oauth/" + code
		return state, verifier, nonce, url, nil
	}
	oauthInfo, oauthConfig, _, err := os.GetOauthConfig(op)
	if err == nil {
		extras := make([]oauth2.AuthCodeOption, 0, 3)

		nonce = utils.RandomString(10)
		extras = append(extras, oauth2.SetAuthURLParam("nonce", nonce))

		if oauthInfo.PkceEnable != nil && *oauthInfo.PkceEnable {
			extras = append(extras, oauth2.AccessTypeOffline)
			verifier = oauth2.GenerateVerifier()
			switch oauthInfo.PkceMethod {
			case model.PKCEMethodS256:
				extras = append(extras, oauth2.S256ChallengeOption(verifier))
			case model.PKCEMethodPlain:
				// oauth2 does not have a plain challenge option, so we add it manually
				extras = append(extras, oauth2.SetAuthURLParam("code_challenge_method", "plain"), oauth2.SetAuthURLParam("code_challenge", verifier))
			}
		}

		return state, verifier, nonce, oauthConfig.AuthCodeURL(state, extras...), nil
	}

	return state, verifier, nonce, "", err
}

func (os *OauthService) FetchOidcProvider(issuer string) (*oidc.Provider, error) {

	// Get the HTTP client (with or without proxy based on configuration)
	client := getHTTPClientWithProxy()

	ctx := oidc.ClientContext(context.Background(), client)

	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}

	return provider, nil
}

// GetOauthConfig retrieves the OAuth2 configuration based on the provider name
func (os *OauthService) GetOauthConfig(op string) (oauthInfo *model.Oauth, oauthConfig *oauth2.Config, provider *oidc.Provider, err error) {
	oauthInfo, err = os.InfoByOp(op)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, nil, nil, err
	}
	// Un provider che non e' oidc, come github, google e linuxdo tolti (A3),
	// resta nel database ma al login risponde come un op che non esiste.
	if err != nil || oauthInfo.OauthType != model.OauthTypeOidc || oauthInfo.ClientId == "" || oauthInfo.ClientSecret == "" {
		return nil, nil, nil, errors.New("ConfigNotFound")
	}
	oauthConfig = &oauth2.Config{
		ClientID:     oauthInfo.ClientId,
		ClientSecret: oauthInfo.ClientSecret,
		RedirectURL:  Config.Rustdesk.ApiServer + "/api/oidc/callback",
	}

	provider, err = os.FetchOidcProvider(oauthInfo.Issuer)
	if err != nil {
		return nil, nil, nil, err
	}
	oauthConfig.Endpoint = provider.Endpoint()
	oauthConfig.Scopes = os.constructScopes(oauthInfo.Scopes)
	return oauthInfo, oauthConfig, provider, nil
}

func getHTTPClientWithProxy() *http.Client {
	// Timeout di 60 secondi, solo col proxy: senza, il client e'
	// http.DefaultClient, che non ne ha (il callback OIDC ha il suo,
	// tempoProviderOidc).
	timeout := time.Duration(60) * time.Second
	if Config.Proxy.Enable {
		if Config.Proxy.Host == "" {
			Logger.Warn("Proxy is enabled but proxy host is empty.")
			return http.DefaultClient
		}
		proxyURL, err := url.Parse(Config.Proxy.Host)
		if err != nil {
			Logger.Warn("Invalid proxy URL: ", err)
			return http.DefaultClient
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		return &http.Client{Transport: transport, Timeout: timeout}
	}
	return http.DefaultClient
}

// tempoProviderOidc e' il tempo massimo delle richieste del callback OIDC al
// provider, dallo scambio del codice alla userinfo: senza, un provider che
// non risponde terrebbe ferma la pagina del login per sempre, perche' senza
// proxy il client HTTP non ha timeout. I test lo accorciano.
var tempoProviderOidc = 30 * time.Second

func (os *OauthService) callbackBase(oauthConfig *oauth2.Config, provider *oidc.Provider, code string, verifier string, nonce string, userData interface{}) error {

	// 设置代理客户端
	httpClient := getHTTPClientWithProxy()
	ctx, cancel := context.WithTimeout(context.Background(), tempoProviderOidc)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	exchangeOpts := make([]oauth2.AuthCodeOption, 0, 1)
	if verifier != "" {
		exchangeOpts = append(exchangeOpts, oauth2.VerifierOption(verifier))
	}

	token, err := oauthConfig.Exchange(ctx, code, exchangeOpts...)

	if err != nil {
		Logger.Warn("oauthConfig.Exchange() failed: ", err)
		return errors.New("GetOauthTokenError")
	}

	// Senza id_token si saltano la verifica del token e quella del nonce: la
	// tolleranza era per GitHub e Linux.do, che non sono OIDC e sono usciti
	// (A3).
	rawIDToken, ok := token.Extra("id_token").(string)
	if ok && rawIDToken != "" {
		// 验证 ID Token
		v := provider.Verifier(&oidc.Config{ClientID: oauthConfig.ClientID})
		idToken, err2 := v.Verify(ctx, rawIDToken)
		if err2 != nil {
			Logger.Warn("IdTokenVerifyError: ", err2)
			return errors.New("IdTokenVerifyError")
		}
		if nonce != "" {
			// 验证 nonce
			var claims struct {
				Nonce string `json:"nonce"`
			}
			if err2 = idToken.Claims(&claims); err2 != nil {
				Logger.Warn("Failed to parse ID Token claims: ", err)
				return errors.New("IDTokenClaimsError")
			}

			if claims.Nonce != nonce {
				Logger.Warn("Nonce does not match")
				return errors.New("NonceDoesNotMatch")
			}
		}
	}

	// 获取用户信息
	client := oauthConfig.Client(ctx, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.UserInfoEndpoint(), nil)
	if err != nil {
		Logger.Warn("richiesta della userinfo non preparata: ", err)
		return errors.New("GetOauthUserInfoError")
	}
	resp, err := client.Do(req)
	if err != nil {
		Logger.Warn("failed getting user info: ", err)
		return errors.New("GetOauthUserInfoError")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			Logger.Warn("failed closing response body: ", closeErr)
		}
	}()

	// 解析用户信息
	if err = json.NewDecoder(resp.Body).Decode(userData); err != nil {
		Logger.Warn("failed decoding user info: ", err)
		return errors.New("DecodeOauthUserInfoError")
	}

	return nil
}

// oidcCallback oidc回调, 通过code获取用户信息
func (os *OauthService) oidcCallback(oauthConfig *oauth2.Config, provider *oidc.Provider, code, verifier, nonce string) (*model.OauthUser, error) {
	var user = &model.OidcUser{}
	if err := os.callbackBase(oauthConfig, provider, code, verifier, nonce, user); err != nil {
		return nil, err
	}
	return user.ToOauthUser(), nil
}

// Callback gets the user information by code and op (the OAuth provider).
func (os *OauthService) Callback(code, verifier, op, nonce string) (oauthUser *model.OauthUser, err error) {
	_, oauthConfig, provider, err := os.GetOauthConfig(op)
	if err != nil {
		return nil, err
	}
	return os.oidcCallback(oauthConfig, provider, code, verifier, nonce)
}

// UserThirdInfo restituisce l'associazione all'account openId del provider
// op; ErrNotFound se non c'e'.
func (os *OauthService) UserThirdInfo(op string, openId string) (*model.UserThird, error) {
	ut := &model.UserThird{}
	if err := DB.Where("open_id = ? and op = ?", openId, op).First(ut).Error; err != nil {
		return nil, fmt.Errorf("associazione al provider: %w", nonTrovato(err))
	}
	return ut, nil
}

// BindOauthUser binds a third party account.
func (os *OauthService) BindOauthUser(userId uint, oauthUser *model.OauthUser, op string) error {
	utr := &model.UserThird{}
	oauthType, err := os.GetTypeByOp(op)
	if err != nil {
		return err
	}
	utr.FromOauthUser(userId, oauthUser, oauthType, op)
	return DB.Create(utr).Error
}

// UnBindOauthUser unbinds a third party account.
func (os *OauthService) UnBindOauthUser(userId uint, op string) error {
	return os.UnBindThird(op, userId)
}

// UnBindThird unbinds a third party account.
func (os *OauthService) UnBindThird(op string, userId uint) error {
	return DB.Where("user_id = ? and op = ?", userId, op).Delete(&model.UserThird{}).Error
}

// DeleteUserByUserId deletes all the third party bindings of a deleted user.
func (os *OauthService) DeleteUserByUserId(userId uint) error {
	return DB.Where("user_id = ?", userId).Delete(&model.UserThird{}).Error
}

// InfoById restituisce il provider OAuth id; ErrNotFound se non c'e'.
func (os *OauthService) InfoById(id uint) (*model.Oauth, error) {
	oauthInfo := &model.Oauth{}
	if err := DB.Where("id = ?", id).First(oauthInfo).Error; err != nil {
		return nil, fmt.Errorf("provider OAuth %d: %w", id, nonTrovato(err))
	}
	return oauthInfo, nil
}

// InfoByOp restituisce il provider OAuth op; ErrNotFound se non c'e'.
func (os *OauthService) InfoByOp(op string) (*model.Oauth, error) {
	oauthInfo := &model.Oauth{}
	if err := DB.Where("op = ?", op).First(oauthInfo).Error; err != nil {
		return nil, fmt.Errorf("provider OAuth %q: %w", op, nonTrovato(err))
	}
	return oauthInfo, nil
}

// Helper function to construct scopes
func (os *OauthService) constructScopes(scopes string) []string {
	scopes = strings.TrimSpace(scopes)
	if scopes == "" {
		scopes = model.OIDC_DEFAULT_SCOPES
	}
	return strings.Split(scopes, ",")
}

func (os *OauthService) List(page, pageSize uint, where func(tx *gorm.DB)) (res *model.OauthList, err error) {
	res = &model.OauthList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Oauth{})
	if where != nil {
		where(tx)
	}
	if err = tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio dei provider OAuth: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err = tx.Find(&res.Oauths).Error; err != nil {
		return nil, fmt.Errorf("provider OAuth: %w", err)
	}
	return res, nil
}

// GetTypeByOp 根据op获取OauthType
func (os *OauthService) GetTypeByOp(op string) (string, error) {
	oauthInfo := &model.Oauth{}
	if DB.Where("op = ?", op).First(oauthInfo).Error != nil {
		return "", fmt.Errorf("OAuth provider with op '%s' not found", op)
	}
	return oauthInfo.OauthType, nil
}

// ValidateOauthProvider 验证Oauth提供者是否正确
func (os *OauthService) ValidateOauthProvider(op string) error {
	if !os.IsOauthProviderExist(op) {
		return fmt.Errorf("OAuth provider with op '%s' not found", op)
	}
	return nil
}

// IsOauthProviderExist 验证Oauth提供者是否存在
func (os *OauthService) IsOauthProviderExist(op string) bool {
	oauthInfo := &model.Oauth{}
	// 使用 Gorm 的 Take 方法查找符合条件的记录
	if err := DB.Where("op = ?", op).Take(oauthInfo).Error; err != nil {
		return false
	}
	return true
}

// Create 创建
func (os *OauthService) Create(oauthInfo *model.Oauth) error {
	err := oauthInfo.FormatOauthInfo()
	if err != nil {
		return err
	}
	res := DB.Create(oauthInfo).Error
	return res
}
func (os *OauthService) Delete(oauthInfo *model.Oauth) error {
	return DB.Delete(oauthInfo).Error
}

// Update salva i campi del modulo del pannello (admin.OauthForm), anche
// vuoti: issuer e scope svuotati restano vuoti. Op, PKCE e autoregistrazione
// vuoti prendono prima i default di FormatOauthInfo, come oggi; client_id e
// client_secret il validatore non li accetta vuoti. La data di creazione
// resta quella di prima.
func (os *OauthService) Update(oauthInfo *model.Oauth) error {
	err := oauthInfo.FormatOauthInfo()
	if err != nil {
		return err
	}
	campi := []string{"op", "oauth_type", "client_id", "client_secret", "auto_register", "scopes", "issuer", "pkce_enable", "pkce_method"}
	if err := DB.Model(oauthInfo).Select(campi).Updates(oauthInfo).Error; err != nil {
		return fmt.Errorf("provider OAuth %d: %w", oauthInfo.Id, err)
	}
	return nil
}

// GetOauthProviders restituisce gli op dei provider di tipo oidc, gli unici
// che il login usa: un provider di un altro tipo non si offre.
func (os *OauthService) GetOauthProviders() ([]string, error) {
	var res []string
	if err := DB.Model(&model.Oauth{}).Where("oauth_type = ?", model.OauthTypeOidc).Pluck("op", &res).Error; err != nil {
		return nil, fmt.Errorf("elenco dei provider OAuth: %w", err)
	}
	return res, nil
}

// NonSupportati restituisce i provider del database che il login ignora
// perche' non sono di tipo oidc, come quelli github, google e linuxdo.
func (os *OauthService) NonSupportati() ([]*model.Oauth, error) {
	var res []*model.Oauth
	if err := DB.Where("oauth_type IS NULL OR oauth_type <> ?", model.OauthTypeOidc).Order("id").Find(&res).Error; err != nil {
		return nil, fmt.Errorf("lettura dei provider OAuth non supportati: %w", err)
	}
	return res, nil
}
