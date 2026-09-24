package global

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// chiaviDi restituisce gli ID dei messaggi del file di lingua path.
func chiaviDi(t *testing.T, path string) map[string]bool {
	t.Helper()
	var messaggi map[string]any
	if _, err := toml.DecodeFile(path, &messaggi); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	chiavi := make(map[string]bool, len(messaggi))
	for id := range messaggi {
		chiavi[id] = true
	}
	return chiavi
}

// TestIDDeiMessaggi verifica che ogni ID che il codice passa a go-i18n sia in
// en.toml, perche' un ID che non c'e' non arriva mai tradotto all'utente: i
// letterali di TranslateMsg, TranslateTempMsg, TranslateParamMsg,
// i18n.Message{ID} e i18n.LocalizeConfig{MessageID}, e le stringhe di
// errors.New e dei tag label che sono un ID (una parola CamelCase), che i
// controller passano a TranslateMsg con err.Error() e il validatore traduce.
func TestIDDeiMessaggi(t *testing.T) {
	en := chiaviDi(t, "../resources/i18n/en.toml")
	unaParola := regexp.MustCompile(`^[A-Z][A-Za-z0-9]+$`)
	traduce := map[string]bool{"TranslateMsg": true, "TranslateTempMsg": true, "TranslateParamMsg": true}
	campoID := map[string]string{"Message": "ID", "LocalizeConfig": "MessageID"}
	fset := token.NewFileSet()
	// controlla segnala id se manca da en.toml; se non e' sempre un ID, solo
	// quando ne ha la forma.
	controlla := func(pos token.Pos, id string, sempre bool) {
		if (sempre || unaParola.MatchString(id)) && !en[id] {
			t.Errorf("%s: l'ID %q non c'e' in en.toml", fset.Position(pos), id)
		}
	}
	letterale := func(e ast.Expr, sempre bool) {
		if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			id, _ := strconv.Unquote(lit.Value)
			controlla(lit.Pos(), id, sempre)
		}
	}
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && (d.Name() == ".git" || d.Name() == "docs" || d.Name() == "resources"):
			return filepath.SkipDir
		case d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go"):
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				switch fun := n.Fun.(type) {
				case *ast.Ident:
					if traduce[fun.Name] && len(n.Args) > 1 {
						letterale(n.Args[1], true)
					}
				case *ast.SelectorExpr:
					if traduce[fun.Sel.Name] && len(n.Args) > 1 {
						letterale(n.Args[1], true)
					} else if x, ok := fun.X.(*ast.Ident); ok && x.Name == "errors" && fun.Sel.Name == "New" {
						letterale(n.Args[0], false)
					}
				}
			case *ast.CompositeLit:
				if tipo, ok := n.Type.(*ast.SelectorExpr); ok && campoID[tipo.Sel.Name] != "" {
					for _, e := range n.Elts {
						if kv, ok := e.(*ast.KeyValueExpr); ok {
							if k, ok := kv.Key.(*ast.Ident); ok && k.Name == campoID[tipo.Sel.Name] {
								letterale(kv.Value, true)
							}
						}
					}
				}
			case *ast.Field:
				if n.Tag != nil {
					tag, _ := strconv.Unquote(n.Tag.Value)
					controlla(n.Tag.Pos(), reflect.StructTag(tag).Get("label"), false)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestChiaviDeiFileDiLingua verifica che it.toml abbia esattamente le chiavi
// di en.toml e che nessun file di lingua ne abbia una che en.toml non ha: gli
// ID nascono in en.toml, e una lingua a cui ne manca uno ripiega
// sull'inglese.
func TestChiaviDeiFileDiLingua(t *testing.T) {
	en := chiaviDi(t, "../resources/i18n/en.toml")
	it := chiaviDi(t, "../resources/i18n/it.toml")
	for id := range en {
		if !it[id] {
			t.Errorf("it.toml: manca %s", id)
		}
	}
	files, err := filepath.Glob("../resources/i18n/*.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		for id := range chiaviDi(t, path) {
			if !en[id] {
				t.Errorf("%s: %s non c'e' in en.toml", filepath.Base(path), id)
			}
		}
	}
}
