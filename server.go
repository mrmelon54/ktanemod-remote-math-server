package ktanemod_remote_math_server

import (
	"errors"
	exitReload "github.com/MrMelon54/exit-reload"
	"github.com/gorilla/websocket"
	"log"
	"math/rand"
	"net/http"
	"time"
)

var upgrader = websocket.Upgrader{}

type Server struct {
	Listen string
	rm     *RemoteMath
}

func (s *Server) Run() {
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	s.rm = NewRemoteMath(random)

	r := http.NewServeMux()
	r.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		if isWebsocketRequest(req) {
			c, err := upgrader.Upgrade(rw, req, nil)
			if err != nil {
				log.Println("[Websocket] Upgrade error: ", err)
				return
			}
			s.websocketHandler(c)
			return
		}
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("What is a \"Remote Math\" anyway?\n"))
	})
	srv := &http.Server{Addr: s.Listen, Handler: r}
	log.Printf("[RemoteMath] Hosting Remote Math on '%s'\n", srv.Addr)
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				log.Println("[RemoteMath] The http server shutdown successfully")
			} else {
				log.Println("[RemoteMath] Error trying to host the http server: ", err)
			}
		}
	}()
	exitReload.ExitReload("RemoteMath", func() {}, func() {
		_ = srv.Close()
		s.rm.Close()
	})
}

func (s *Server) websocketHandler(c *websocket.Conn) {
	defer c.Close()

	// state value
	//
	//   0 = new connection
	//   1 = module client
	//   2 = web client pre-connect
	//   3 = web client post-connect
	var state int
	var puzzle *Puzzle

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			break
		}
		if mt != websocket.TextMessage {
			break
		}
		switch state {
		case 0:
			switch string(message) {
			case "blåhaj":
				state = 1
				puzzle = s.rm.CreatePuzzle(c)
				puzzle.SendMod("ClientSelected")
				puzzle.SendMod("PuzzleCode::" + puzzle.code)
				puzzle.SendMod("PuzzleLog::" + puzzle.date.Format(time.DateOnly) + "/" + puzzle.code)
			case "rin":
				state = 2
				puzzle.SendMod("ClientSelected")
			}
		case 1:
			puzzle.RecvMod(string(message))
		case 2:
			puzzle = s.rm.ConnectPuzzle(c, string(message))
			if puzzle == nil {
				_ = c.Close()
				return
			}
			state = 3
		case 3:
			puzzle.RecvWebConn(string(message))
		}
	}
	switch state {
	case 1:
		s.rm.ClosePuzzle(puzzle)
	case 3:
		puzzle.RemoveWebConn(c)
	}
}

func isWebsocketRequest(req *http.Request) bool {
	return req.Header.Get("Connection") == "upgrade" && req.Header.Get("Upgrade") == "websocket"
}
