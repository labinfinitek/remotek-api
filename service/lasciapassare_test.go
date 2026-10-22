package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
)

// semeDiProva e' il seme del vettore del README: i byte da 0 a 31.
func semeDiProva() ed25519.PrivateKey {
	s := make([]byte, ed25519.SeedSize)
	for i := range s {
		s[i] = byte(i)
	}
	return ed25519.NewKeyFromSeed(s)
}

// TestLasciapassareVettore fissa il vettore del README: Ed25519 e'
// deterministica, quindi seme e payload fissi danno sempre la stessa
// stringa, che il client usa per provare la sua verifica. La firma si
// verifica con la chiave pubblica sul prefisso piu' il payload.
func TestLasciapassareVettore(t *testing.T) {
	p := PayloadLasciapassare{V: 1, Pc: "999000111", Agente: "agente-mario", Tecnico: "mario", Modo: ModoAmministratore,
		Emesso: 1791000000, Scade: 1791000060, Nonce: "AAECAwQFBgcICQoLDA0ODw"}
	got, err := FirmaLasciapassare(semeDiProva(), p)
	if err != nil {
		t.Fatal(err)
	}
	const want = "eyJ2IjoxLCJwYyI6Ijk5OTAwMDExMSIsImFnZW50ZSI6ImFnZW50ZS1tYXJpbyIsInRlY25pY28iOiJtYXJpbyIsIm1vZG8iOiJhbW1pbmlzdHJhdG9yZSIsImVtZXNzbyI6MTc5MTAwMDAwMCwic2NhZGUiOjE3OTEwMDAwNjAsIm5vbmNlIjoiQUFFQ0F3UUZCZ2NJQ1FvTERBME9EdyJ9" +
		".BRkkm77ln4n8PxwF7Jmi3_-LiCHX7xGjLFjiXb3Knpc3kg9Rlv6DNoxForK3F28DLaUF5HWkudV4ZaWexK4YDA"
	if got != want {
		t.Errorf("FirmaLasciapassare:\n got  %s\n want %s", got, want)
	}
	payload, firma, ok := strings.Cut(got, ".")
	pb, err1 := base64.RawURLEncoding.DecodeString(payload)
	fb, err2 := base64.RawURLEncoding.DecodeString(firma)
	if !ok || err1 != nil || err2 != nil {
		t.Fatalf("formato: %q (%v, %v)", got, err1, err2)
	}
	const json = `{"v":1,"pc":"999000111","agente":"agente-mario","tecnico":"mario","modo":"amministratore",` +
		`"emesso":1791000000,"scade":1791000060,"nonce":"AAECAwQFBgcICQoLDA0ODw"}`
	if string(pb) != json {
		t.Errorf("payload:\n got  %s\n want %s", pb, json)
	}
	pub := semeDiProva().Public().(ed25519.PublicKey)
	if !ed25519.Verify(pub, []byte(PrefissoLasciapassare+json), fb) {
		t.Error("la firma non si verifica con la chiave pubblica")
	}
	if got := base64.StdEncoding.EncodeToString(pub); got != "A6EHv/POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg=" {
		t.Errorf("chiave pubblica del vettore %s", got)
	}
}
