package main

import (
	"io"
	"net/http"

	"golang.org/x/net/websocket"
)

func main() {
	handler := func(ws *websocket.Conn) {
		io.Copy(ws, ws)
	}

	http.Handle("/v1/outlet", websocket.Handler(handler))
	if err := http.ListenAndServeTLS(":8005", "./certificate.pem", "./key.pem", nil); err != nil {
		panic(err)
	}
}
