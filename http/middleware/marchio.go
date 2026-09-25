package middleware

import "github.com/gin-gonic/gin"

// politicaMarchio e' la Content-Security-Policy dei file del marchio: un SVG
// aperto direttamente e' un documento nell'origine del pannello, quindi
// niente script, niente richieste fuori da immagini e font della stessa
// origine o in data:, stili solo dentro il file, e sandbox.
const politicaMarchio = "default-src 'none'; img-src 'self' data:; font-src 'self' data:; style-src 'unsafe-inline'; sandbox"

// Marchio mette sulle risposte di /brand/* la politicaMarchio e
// X-Content-Type-Options: nosniff, perche' il browser usi il tipo che
// l'API dichiara e non lo indovini dal contenuto.
func Marchio() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", politicaMarchio)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	}
}
