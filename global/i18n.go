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
	scegli := sceltaLingua()
	Localizer = func(lang string) *i18n.Localizer {
		return i18n.NewLocalizer(bundle, scegli(lang).String())
	}
}

// sceltaLingua restituisce la regola, una sola per messaggi e validatore,
// che sceglie la lingua di una risposta da Accept-Language: la prima lingua
// dell'intestazione che l'API ha; altrimenti Config.Lang, se l'API la ha;
// altrimenti l'inglese. Il pannello manda la sua lingua, di partenza quella
// del browser ("it-IT"); il client RustDesk non manda Accept-Language.
func sceltaLingua() func(acceptLanguage string) language.Tag {
	var tags []language.Tag
	for _, l := range lingue() {
		tags = append(tags, l.tag)
	}
	matcher := language.NewMatcher(tags)
	return func(acceptLanguage string) language.Tag {
		for _, voluta := range []string{acceptLanguage, Config.Lang} {
			voluti, _, err := language.ParseAcceptLanguage(voluta)
			if err != nil || len(voluti) == 0 {
				continue
			}
			if _, i, conf := matcher.Match(voluti...); conf != language.No {
				return tags[i]
			}
		}
		return language.English
	}
}
