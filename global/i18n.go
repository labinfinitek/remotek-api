package global

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func InitI18n() {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	// Carica i file .toml di resources/i18n. Uno che non si carica ferma
	// l'avvio: saltato, l'API risponderebbe in inglese senza dirlo a nessuno.
	dir := filepath.Join(Config.Gin.ResourcesPath, "i18n")
	fileInfos, err := os.ReadDir(dir)
	if err != nil {
		Logger.Fatalf("cartella dei file di lingua non leggibile, l'API non parte: %v", err)
	}
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() || !strings.HasSuffix(fileInfo.Name(), ".toml") {
			continue
		}
		if _, err := bundle.LoadMessageFile(filepath.Join(dir, fileInfo.Name())); err != nil {
			Logger.Fatalf("file di lingua %s non caricato, l'API non parte: %v", fileInfo.Name(), err)
		}
	}
	Localizer = func(lang string) *i18n.Localizer {
		if lang == "" {
			lang = Config.Lang
		}
		if lang == "en" {
			return i18n.NewLocalizer(bundle, "en")
		} else {
			return i18n.NewLocalizer(bundle, lang, "en")
		}
	}

	//personUnreadEmails := localizer.MustLocalize(&i18n.LocalizeConfig{
	//	DefaultMessage: &i18n.Message{
	//		ID: "PersonUnreadEmails",
	//	},
	//	PluralCount: 6,
	//	TemplateData: map[string]interface{}{
	//		"Name":        "LE",
	//		"PluralCount": 6,
	//	},
	//})
	//personUnreadEmails, err := global.Localizer.LocalizeMessage(&i18n.Message{
	//	ID: "ParamsError",
	//})
	//fmt.Println(err, personUnreadEmails)

}
