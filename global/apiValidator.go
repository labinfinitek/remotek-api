package global

import (
	"errors"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/locales"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/es"
	"github.com/go-playground/locales/fr"
	"github.com/go-playground/locales/it"
	"github.com/go-playground/locales/ko"
	"github.com/go-playground/locales/ru"
	"github.com/go-playground/locales/zh_Hans_CN"
	"github.com/go-playground/locales/zh_Hant"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	es_translations "github.com/go-playground/validator/v10/translations/es"
	fr_translations "github.com/go-playground/validator/v10/translations/fr"
	it_translations "github.com/go-playground/validator/v10/translations/it"
	ko_translations "github.com/go-playground/validator/v10/translations/ko"
	ru_translations "github.com/go-playground/validator/v10/translations/ru"
	zh_translations "github.com/go-playground/validator/v10/translations/zh"
	zh_tw_translations "github.com/go-playground/validator/v10/translations/zh_tw"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// lingua e' una lingua dell'API: ha un file in resources/i18n e le
// traduzioni dei messaggi del validatore.
type lingua struct {
	tag        language.Tag
	locale     func() locales.Translator
	traduzioni func(*validator.Validate, ut.Translator) error
}

// lingue restituisce le lingue dell'API, una per file di lingua.
func lingue() []lingua {
	return []lingua{
		{language.English, en.New, en_translations.RegisterDefaultTranslations},
		{language.Italian, it.New, it_translations.RegisterDefaultTranslations},
		{language.Spanish, es.New, es_translations.RegisterDefaultTranslations},
		{language.French, fr.New, fr_translations.RegisterDefaultTranslations},
		{language.Korean, ko.New, ko_translations.RegisterDefaultTranslations},
		{language.Russian, ru.New, ru_translations.RegisterDefaultTranslations},
		{language.MustParse("zh-CN"), zh_Hans_CN.New, zh_translations.RegisterDefaultTranslations},
		{language.MustParse("zh-TW"), zh_Hant.New, zh_tw_translations.RegisterDefaultTranslations},
	}
}

// ApiInitValidator prepara il validatore delle richieste, uno per lingua:
// il validatore tiene in cache per tipo il nome dei campi, e il nome deve
// uscire nella lingua della risposta. La lingua la sceglie la stessa regola
// dei messaggi (sceltaLingua).
func ApiInitValidator() {
	type validatore struct {
		v *validator.Validate
		t ut.Translator
	}
	perLingua := make(map[language.Tag]validatore)
	for _, l := range lingue() {
		loc := l.locale()
		t, _ := ut.New(loc, loc).GetTranslator(loc.Locale())
		v := validator.New()
		if err := l.traduzioni(v, t); err != nil {
			panic(err)
		}
		v.RegisterTagNameFunc(nomeCampo(l.tag))
		perLingua[l.tag] = validatore{v, t}
	}
	scegli := sceltaLingua()
	valida := func(ctx *gin.Context, controlla func(*validator.Validate) error) []string {
		l := perLingua[scegli(ctx.GetHeader("Accept-Language"))]
		var errs validator.ValidationErrors
		if err := controlla(l.v); !errors.As(err, &errs) {
			if err != nil {
				return []string{err.Error()}
			}
			return nil
		}
		msgs := make([]string, 0, len(errs))
		for _, fe := range errs {
			msgs = append(msgs, fe.Translate(l.t))
		}
		return msgs
	}
	Validator.ValidStruct = func(ctx *gin.Context, i interface{}) []string {
		return valida(ctx, func(v *validator.Validate) error { return v.Struct(i) })
	}
	Validator.ValidVar = func(ctx *gin.Context, field interface{}, tag string) []string {
		return valida(ctx, func(v *validator.Validate) error { return v.Var(field, tag) })
	}
}

// nomeCampo da' al validatore della lingua tag il nome di un campo nei
// messaggi: il tag label, un ID dei file di lingua, tradotto (com'e' se non
// e' un ID o se i file di lingua non sono ancora caricati); senza label, il
// nome Go del campo.
func nomeCampo(tag language.Tag) validator.TagNameFunc {
	return func(campo reflect.StructField) string {
		label := campo.Tag.Get("label")
		if label == "" {
			return campo.Name
		}
		if Localizer != nil {
			if nome, _ := Localizer(tag.String()).LocalizeMessage(&i18n.Message{ID: label}); nome != "" {
				return nome
			}
		}
		return label
	}
}
