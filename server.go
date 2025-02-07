package ktanemod_remote_math_server

import (
	"context"
	"errors"
	"fmt"
	exitReload "github.com/MrMelon54/exit-reload"
	"github.com/gorilla/websocket"
	"log"
	"math/rand"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(req *http.Request) bool {
		h := req.URL.Hostname()
		return h == "remote-math.mrmelon54.com" || h == "localhost" || h == "127.0.0.1" || h == ""
	},
}

var regLogDate = regexp.MustCompile("^[0-9]{4}-[0-9]{2}-[0-9]{2}$")
var regLogCode = regexp.MustCompile("^[a-zA-Z]{6}$")

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
	s.pingStop = make(chan struct{}, 1)
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

	r.HandleFunc("/log", func(rw http.ResponseWriter, req *http.Request) {
		q := req.URL.Query()
		date := q.Get("date")
		code := q.Get("code")
		if !regLogDate.MatchString(date) || !regLogCode.MatchString(code) {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		logFile := filepath.Join(s.LogDir, date, strings.ToUpper(code)+".log")
		if strings.Contains(logFile, "..") || !strings.HasPrefix(logFile, s.LogDir) {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeFile(rw, req, logFile)
	})

	// setup http listener
	srv := &http.Server{
		Addr:              s.Listen,
		Handler:           r,
		ReadTimeout:       time.Minute,
		ReadHeaderTimeout: time.Minute,
		WriteTimeout:      time.Minute,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    2500,
	}
	log.Printf("[RemoteMath] Hosting Remote Math on '%s'\n", srv.Addr)
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				log.Println("[RemoteMath] The http server shutdown successfully")
			} else {
				log.Fatalln("[RemoteMath] Error trying to host the http server: ", err)
			}
		}
	}()
	exitReload.ExitReload("RemoteMath", func() {}, func() {
		close(s.pingStop)

		// close all websockets connections
		s.mLock.Lock()
		fmt.Printf("Closing %d connections\n", len(s.m))
		for _, i := range s.m {
			fmt.Printf("Closing connection %s, %s, %s\n", i.LocalAddr(), i.RemoteAddr(), i.Subprotocol())
			_ = i.Close()
			fmt.Println("Closed")
		}
		s.m = make(map[string]*websocket.Conn)
		s.mLock.Unlock()

		// close remote math handler
		s.rm.Close()
		_ = srv.Shutdown(context.Background())
	})
}

func (s *Server) StartPinger() {
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
	outer:
		for {
			select {
			case <-s.pingStop:
				break outer
			case <-t.C:
				s.mLock.RLock()
				for _, v := range s.m {
					if v == nil {
						continue
					}
					_ = v.WriteMessage(websocket.TextMessage, []byte("ping"))
				}
				s.mLock.RUnlock()
			}
		}
		fmt.Println("[Remote Math] Background ping sender stopped")
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
				puzzle.SendMod("PuzzleLog::LogFile/" + puzzle.date.Format(time.DateOnly) + "/" + puzzle.code)
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
	case ModuleClient:
		s.rm.ClosePuzzle(puzzle)
	case WebClientPostConnect:
		puzzle.RemoveWebConn(c)
	}
}
