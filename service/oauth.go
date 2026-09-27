package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/utils"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	// "io"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type OauthService struct {
}

// Define a struct to parse the .well-known/openid-configuration response
type OidcEndpoint struct {
	Issuer   string `json:"issuer"`
	AuthURL  string `json:"authorization_endpoint"`
	TokenURL string `json:"token_endpoint"`
	UserInfo string `json:"userinfo_endpoint"`
}

type OauthCacheItem struct {
	UserId     uint   `json:"user_id"`
	Id         string `json:"id"` //rustdesk的设备ID
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

func (os *OauthService) BeginAuth(op string) (error error, state, verifier, nonce, url string) {
	state = utils.RandomString(10) + strconv.FormatInt(time.Now().Unix(), 10)
	verifier = ""
	nonce = ""
	if op == model.OauthTypeWebauth {
		// Con app.web-sso spento webauth non esiste: stessa risposta di un op
		// che non c'e', non solo la voce tolta da /api/login-options.
		if !Config.App.WebSso {
			return errors.New("ConfigNotFound"), state, verifier, nonce, ""
		}
		url = Config.Rustdesk.ApiServer + "/_admin/#/oauth/" + state
		//url = "http://localhost:8888/_admin/#/oauth/" + code
		return nil, state, verifier, nonce, url
	}
	err, oauthInfo, oauthConfig, _ := os.GetOauthConfig(op)
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

		return err, state, verifier, nonce, oauthConfig.AuthCodeURL(state, extras...)
	}

	return err, state, verifier, nonce, ""
}

func (os *OauthService) FetchOidcProvider(issuer string) (error, *oidc.Provider) {

	// Get the HTTP client (with or without proxy based on configuration)
	client := getHTTPClientWithProxy()

	ctx := oidc.ClientContext(context.Background(), client)

	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return err, nil
	}

	return nil, provider
}

// GetOauthConfig retrieves the OAuth2 configuration based on the provider name
func (os *OauthService) GetOauthConfig(op string) (err error, oauthInfo *model.Oauth, oauthConfig *oauth2.Config, provider *oidc.Provider) {
	//err, oauthInfo, oauthConfig = os.getOauthConfigGeneral(op)
	oauthInfo = os.InfoByOp(op)
	// Un provider che non e' oidc, come github, google e linuxdo tolti (A3),
	// resta nel database ma al login risponde come un op che non esiste.
	if oauthInfo.Id == 0 || oauthInfo.OauthType != model.OauthTypeOidc || oauthInfo.ClientId == "" || oauthInfo.ClientSecret == "" {
		return errors.New("ConfigNotFound"), nil, nil, nil
	}
	oauthConfig = &oauth2.Config{
		ClientID:     oauthInfo.ClientId,
		ClientSecret: oauthInfo.ClientSecret,
		RedirectURL:  Config.Rustdesk.ApiServer + "/api/oidc/callback",
	}

	err, provider = os.FetchOidcProvider(oauthInfo.Issuer)
	if err != nil {
		return err, nil, nil, nil
	}
	oauthConfig.Endpoint = provider.Endpoint()
	oauthConfig.Scopes = os.constructScopes(oauthInfo.Scopes)
	return nil, oauthInfo, oauthConfig, provider
}

func getHTTPClientWithProxy() *http.Client {
	//add timeout 30s
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
func (os *OauthService) callbackBase(oauthConfig *oauth2.Config, provider *oidc.Provider, code string, verifier string, nonce string, userData interface{}) error {

	// 设置代理客户端
	httpClient := getHTTPClientWithProxy()
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

	exchangeOpts := make([]oauth2.AuthCodeOption, 0, 1)
	if verifier != "" {
		exchangeOpts = append(exchangeOpts, oauth2.VerifierOption(verifier))
	}

	token, err := oauthConfig.Exchange(ctx, code, exchangeOpts...)

	if err != nil {
		Logger.Warn("oauthConfig.Exchange() failed: ", err)
		return errors.New("GetOauthTokenError")
	}

	// Senza id_token la verifica si salta, nonce compreso: la tolleranza era
	// per GitHub e Linux.do, che non sono OIDC e sono usciti (A3).
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
	resp, err := client.Get(provider.UserInfoEndpoint())
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
func (os *OauthService) oidcCallback(oauthConfig *oauth2.Config, provider *oidc.Provider, code, verifier, nonce string) (error, *model.OauthUser) {
	var user = &model.OidcUser{}
	if err := os.callbackBase(oauthConfig, provider, code, verifier, nonce, user); err != nil {
		return err, nil
	}
	return nil, user.ToOauthUser()
}

// Callback: Get user information by code and op(Oauth provider)
func (os *OauthService) Callback(code, verifier, op, nonce string) (err error, oauthUser *model.OauthUser) {
	err, _, oauthConfig, provider := os.GetOauthConfig(op)
	if err != nil {
		return err, nil
	}
	return os.oidcCallback(oauthConfig, provider, code, verifier, nonce)
}

func (os *OauthService) UserThirdInfo(op string, openId string) *model.UserThird {
	ut := &model.UserThird{}
	DB.Where("open_id = ? and op = ?", openId, op).First(ut)
	return ut
}

// BindOauthUser: Bind third party account
func (os *OauthService) BindOauthUser(userId uint, oauthUser *model.OauthUser, op string) error {
	utr := &model.UserThird{}
	err, oauthType := os.GetTypeByOp(op)
	if err != nil {
		return err
	}
	utr.FromOauthUser(userId, oauthUser, oauthType, op)
	return DB.Create(utr).Error
}

// UnBindOauthUser: Unbind third party account
func (os *OauthService) UnBindOauthUser(userId uint, op string) error {
	return os.UnBindThird(op, userId)
}

// UnBindThird: Unbind third party account
func (os *OauthService) UnBindThird(op string, userId uint) error {
	return DB.Where("user_id = ? and op = ?", userId, op).Delete(&model.UserThird{}).Error
}

// DeleteUserByUserId: When user is deleted, delete all third party bindings
func (os *OauthService) DeleteUserByUserId(userId uint) error {
	return DB.Where("user_id = ?", userId).Delete(&model.UserThird{}).Error
}

// InfoById 根据id获取Oauth信息
func (os *OauthService) InfoById(id uint) *model.Oauth {
	oauthInfo := &model.Oauth{}
	DB.Where("id = ?", id).First(oauthInfo)
	return oauthInfo
}

// InfoByOp 根据op获取Oauth信息
func (os *OauthService) InfoByOp(op string) *model.Oauth {
	oauthInfo := &model.Oauth{}
	DB.Where("op = ?", op).First(oauthInfo)
	return oauthInfo
}

// Helper function to get scopes by operation
func (os *OauthService) getScopesByOp(op string) []string {
	scopes := os.InfoByOp(op).Scopes
	return os.constructScopes(scopes)
}

// Helper function to construct scopes
func (os *OauthService) constructScopes(scopes string) []string {
	scopes = strings.TrimSpace(scopes)
	if scopes == "" {
		scopes = model.OIDC_DEFAULT_SCOPES
	}
	return strings.Split(scopes, ",")
}

func (os *OauthService) List(page, pageSize uint, where func(tx *gorm.DB)) (res *model.OauthList) {
	res = &model.OauthList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Oauth{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.Oauths)
	return
}

// GetTypeByOp 根据op获取OauthType
func (os *OauthService) GetTypeByOp(op string) (error, string) {
	oauthInfo := &model.Oauth{}
	if DB.Where("op = ?", op).First(oauthInfo).Error != nil {
		return fmt.Errorf("OAuth provider with op '%s' not found", op), ""
	}
	return nil, oauthInfo.OauthType
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

// Update 更新
func (os *OauthService) Update(oauthInfo *model.Oauth) error {
	err := oauthInfo.FormatOauthInfo()
	if err != nil {
		return err
	}
	return DB.Model(oauthInfo).Updates(oauthInfo).Error
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
