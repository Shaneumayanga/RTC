package main

import (
	b64 "encoding/base64"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

type MSG struct {
	Type string //sdp or ice
	Data interface{}
	UUID string
}

type Hub struct {
	Connections map[*websocket.Conn]bool
	NewConns    chan *websocket.Conn
	BroadCast   chan *MSG
}

type H struct{}

var upgrader = websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}
var hub *Hub
var home = template.Must(template.ParseFiles("./static/index.html"))
var voice = template.Must(template.ParseFiles("./static/voice.html"))

func init() {
	hub = &Hub{
		Connections: make(map[*websocket.Conn]bool),
		NewConns:    make(chan *websocket.Conn),
		BroadCast:   make(chan *MSG),
	}
	go func() {
		for {
			select {
			case newconn := <-hub.NewConns:
				hub.Connections[newconn] = true
			case broadCast := <-hub.BroadCast:
				for con := range hub.Connections {
					if err := con.WriteJSON(broadCast); err != nil {
						log.Println(err.Error())
					}
				}
			}
		}
	}()
}

func (h *H) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if r.RequestURI == "/" {
			home.Execute(rw, r)
		}
		if r.RequestURI == "/voice" {
			name, err := r.Cookie("name")
			if err != nil {
				http.Redirect(rw, r, "/", http.StatusSeeOther)
				return
			}
			if name == nil {
				http.Redirect(rw, r, "/", http.StatusSeeOther)
				return
			}
			voice.Execute(rw, r)
		}
		if r.RequestURI == "/ws" {
			conn, err := upgrader.Upgrade(rw, r, rw.Header())
			if err != nil {
				http.Error(rw, "Error", http.StatusInternalServerError)
				return
			}
			hub.NewConns <- conn
			go Read(conn)
		}
	} else if r.Method == http.MethodPost {
		if r.RequestURI == "/login" {
			name := r.FormValue("name")
			name64 := b64.StdEncoding.EncodeToString([]byte(name))
			cookie := &http.Cookie{
				Name:   "name",
				Value:  name64,
				MaxAge: 24 * 60 * 365,
			}
			http.SetCookie(rw, cookie)
			http.Redirect(rw, r, "/voice", http.StatusSeeOther)
		}
	}
}

func Read(ws *websocket.Conn) {
	for {
		msg := &MSG{}
		if err := ws.ReadJSON(msg); err != nil {
			log.Print(err.Error())
			return
		}
		hub.BroadCast <- msg
	}
}

func main() {
	svc := &http.Server{
		Addr:    ":8080",
		Handler: new(H),
	}
	log.Fatal(svc.ListenAndServe())
}
