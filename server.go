package ktanemod_remote_math_server

import (
	"context"
	"errors"
	exitReload "github.com/MrMelon54/exit-reload"
	"github.com/gorilla/websocket"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(req *http.Request) bool {
		h := req.URL.Hostname()
		return h == "remote-math.mrmelon54.com" || h == "localhost" || h == "127.0.0.1" || h == ""
	},
}

type Server struct {
	Listen      string
	LogDir      string
	DebugPuzzle bool
	rm          *RemoteMath
	mLock       *sync.RWMutex
	m           map[string]*websocket.Conn
	pingStop    chan struct{}
}

func (s *Server) Run() {
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	s.rm = NewRemoteMath(random, s.LogDir, s.DebugPuzzle)
	s.mLock = new(sync.RWMutex)
	s.m = make(map[string]*websocket.Conn)
	s.pingStop = make(chan struct{})
	s.StartPinger()

	r := http.NewServeMux()
	r.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		if websocket.IsWebSocketUpgrade(req) {
			log.Printf("[Websocket] Upgrading connection by '%s' from '%s'\n", req.RemoteAddr, req.Header.Get("Origin"))
			c, err := upgrader.Upgrade(rw, req, nil)
			if err != nil {
				log.Println("[Websocket] Upgrade error: ", err)
				return
			}
			s.mLock.Lock()
			s.m[c.RemoteAddr().String()] = c
			s.mLock.Unlock()
			go s.websocketHandler(c)
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
		close(s.pingStop)
		s.rm.Close()
		_ = srv.Shutdown(context.Background())
	})
}

func (s *Server) StartPinger() {
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-s.pingStop:
				return
			case <-t.C:
				s.mLock.Lock()
				for _, v := range s.m {
					if v == nil {
						continue
					}
					_ = v.WriteMessage(websocket.TextMessage, []byte("ping"))
				}
				s.mLock.Unlock()
			}
		}
	}()
}

// State value
//
//   0 = new connection
//   1 = module client
//   2 = web client pre-connect
//   3 = web client post-connect
type State byte

const (
	NewConnection State = iota
	ModuleClient
	WebClientPreConnect
	WebClientPostConnect
)

func (s *Server) websocketHandler(c *websocket.Conn) {
	defer func() {
		s.mLock.Lock()
		delete(s.m, c.RemoteAddr().String())
		s.mLock.Unlock()
		_ = c.Close()
	}()

	var state = NewConnection
	var puzzle *Puzzle

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("[Websocket] Read message error: ", err)
			break
		}
		if mt != websocket.TextMessage {
			break
		}
		switch state {
		case NewConnection:
			if string(message) == "pong" {
				break
			}
			switch string(message) {
			case "blåhaj":
				state = ModuleClient
				_ = c.WriteMessage(websocket.TextMessage, []byte("ClientSelected"))
				puzzle = s.rm.CreatePuzzle(c)
				puzzle.SendMod("PuzzleCode::" + puzzle.code)
				puzzle.SendMod("PuzzleLog::" + puzzle.date.Format(time.DateOnly) + "/" + puzzle.code)
			case "rin":
				state = WebClientPreConnect
				_ = c.WriteMessage(websocket.TextMessage, []byte("ClientSelected"))
			}
		case ModuleClient:
			if string(message) == "pong" {
				break
			}
			puzzle.RecvMod(string(message))
		case WebClientPreConnect:
			if string(message) == "pong" {
				break
			}
			puzzle = s.rm.ConnectPuzzle(c, string(message))
			if puzzle == nil {
				_ = c.Close()
				return
			}
			state = WebClientPostConnect
		case WebClientPostConnect:
			if string(message) == "pong" {
				break
			}
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
