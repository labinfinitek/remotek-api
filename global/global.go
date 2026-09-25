package global

import (
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/lib/lock"
	"github.com/lejianwen/rustdesk-api/v2/utils"
)

var (
	DB         *gorm.DB
	Logger     *logrus.Logger
	ConfigPath string
	Config     config.Config
	Viper      *viper.Viper
	Validator  struct {
		ValidStruct func(*gin.Context, interface{}) []string
		ValidVar    func(ctx *gin.Context, field interface{}, tag string) []string
	}
	Jwt          *jwt.Jwt
	Lock         lock.Locker
	Localizer    func(lang string) *i18n.Localizer
	LoginLimiter *utils.LoginLimiter
)
