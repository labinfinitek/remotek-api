package http

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// daSistemare elenca i file di http/controller che mettono ancora il testo di
// un errore nella risposta, con quante chiamate X.Error() hanno: al loro
// posto vanno response.ErrorErr e response.FailErr, che lo scrivono nel log
// (REGOLE 8). Chi sistema un file ne abbassa il numero o lo toglie.
var daSistemare = map[string]int{
	// /api/oidc/*, aspetta i golden del contratto
	"api/ouath.go": 5,
	// pannello, amministrazione
	"admin/addressBook.go":               11,
	"admin/addressBookCollection.go":     7,
	"admin/addressBookCollectionRule.go": 7,
	"admin/audit.go":                     10,
	"admin/deviceGroup.go":               7,
	"admin/group.go":                     7,
	"admin/login.go":                     5,
	"admin/loginLog.go":                  5,
	"admin/oauth.go":                     14,
	"admin/peer.go":                      10,
	"admin/rustdesk.go":                  9,
	"admin/shareRecord.go":               5,
	"admin/tag.go":                       7,
	"admin/user.go":                      12,
	"admin/userToken.go":                 5,
}

// TestErrorNeiController conta, file per file, le chiamate X.Error() senza
// argomenti nei file .go di http/controller (non nei test) fuori dalle
// chiamate al logger. Il lint non le vede. Fallisce se un file ne ha piu' di
// quante gliene concede daSistemare, e se daSistemare ne concede piu' di
// quante ce ne sono: l'elenco scende a ogni MR, finche' non si svuota.
func TestErrorNeiController(t *testing.T) {
	trovate := make(map[string]int)
	fset := token.NewFileSet()
	err := filepath.WalkDir("controller", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go"):
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		nome := filepath.ToSlash(strings.TrimPrefix(path, "controller"+string(filepath.Separator)))
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if alLogger(call.Fun) {
				return false
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Error" && len(call.Args) == 0 {
				trovate[nome]++
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for nome, n := range trovate {
		if n > daSistemare[nome] {
			t.Errorf("%s: %d chiamate X.Error(), daSistemare ne concede %d: usa response.ErrorErr o response.FailErr", nome, n, daSistemare[nome])
		}
	}
	for nome, concesse := range daSistemare {
		if trovate[nome] < concesse {
			t.Errorf("%s: daSistemare concede %d chiamate X.Error(), ce ne sono %d: abbassa il numero", nome, concesse, trovate[nome])
		}
	}
}

// alLogger dice se fun, la funzione chiamata, e' del logger:
// global.Logger.Warnf, global.Logger.WithField(...).Warn, log.Printf,
// slog.Warn.
func alLogger(fun ast.Expr) bool {
	for {
		switch e := fun.(type) {
		case *ast.SelectorExpr:
			if e.Sel.Name == "Logger" {
				return true
			}
			fun = e.X
		case *ast.CallExpr:
			fun = e.Fun
		case *ast.Ident:
			return e.Name == "log" || e.Name == "slog"
		default:
			return false
		}
	}
}
