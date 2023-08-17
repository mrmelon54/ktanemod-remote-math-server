package ktanemod_remote_math_server

import (
	"fmt"
	"github.com/gorilla/websocket"
	"math/rand"
	"regexp"
	"sync"
)

var regPuzzleConnect = regexp.MustCompile("^PuzzleConnect::([A-Z]{6})$")

const idBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

type RemoteMath struct {
	rId        *rand.Rand
	puzzleLock *sync.RWMutex
	puzzles    map[string]*Puzzle
	puzzleStop bool
}

func NewRemoteMath(random *rand.Rand) *RemoteMath {
	return &RemoteMath{
		rId:        random,
		puzzleLock: new(sync.RWMutex),
		puzzles:    make(map[string]*Puzzle),
	}
}

func (r *RemoteMath) Close() {
	r.puzzleLock.Lock()
	defer r.puzzleLock.Unlock()
	if r.puzzleStop {
		return
	}
	r.puzzleStop = true
	for _, i := range r.puzzles {
		go i.Kill()
	}
}

func (r *RemoteMath) CreatePuzzle(conn *websocket.Conn) *Puzzle {
	p := NewPuzzle(conn)
	p.cText = [2]int{r.rId.Intn(6), r.rId.Intn(6)}

	// make sure puzzle code is only used once at a time
	r.puzzleLock.Lock()
	if r.puzzleStop {
		r.puzzleLock.Unlock()
		return nil
	}
	p.code = r.genPuzzleCode()
	r.puzzles[p.code] = p
	r.puzzleLock.Unlock()
	p.log.Printf("Module ID: %s\n", p.code)
	return p
}

func (r *RemoteMath) ClosePuzzle(puzzle *Puzzle) {
	r.puzzleLock.Lock()
	if r.puzzleStop {
		r.puzzleLock.Unlock()
		return
	}
	r.puzzles[puzzle.code] = nil
	r.puzzleLock.Unlock()
	puzzle.Kill()
}

func (r *RemoteMath) ConnectPuzzle(c *websocket.Conn, s string) *Puzzle {
	match := regPuzzleConnect.FindStringSubmatch(s)
	if match == nil {
		return nil
	}
	r.puzzleLock.RLock()
	defer r.puzzleLock.RUnlock()
	if r.puzzleStop {
		return nil
	}

	// get puzzle
	p := r.puzzles[match[1]]
	if p == nil {
		return nil
	}

	p.webConnLock.Lock()

	// gen new twitch plays auth code
	var tpCode string
	if p.twitchPlays {
	outer:
		for {
			tpCode = makeId(r.rId, 3, "0123456789")
			for _, i := range p.webConns {
				if i.tpCode == tpCode {
					// skip inner loop as the code already exists
					break
				}
				// skip outer loop as the code is free
				break outer
			}
		}
	}

	// add new web conn
	p.webConns = append(p.webConns, &WebConn{
		conn:   c,
		tpDone: tpCode == "",
		tpCode: tpCode,
	})
	p.webConnLock.Unlock()

	_ = c.WriteMessage(websocket.TextMessage, []byte("PuzzleConnected"))
	_ = c.WriteMessage(websocket.TextMessage, []byte("PuzzleFruits::"+fmt.Sprintf("%d::%d::%d::%d", p.fruits[0], p.fruits[1], p.fruits[2], p.fruits[3])))
	_ = c.WriteMessage(websocket.TextMessage, []byte("PuzzleFruitText::"+fmt.Sprintf("%d::%d", p.cText[0], p.cText[1])))
	if tpCode != "" {
		p.SendMod("PuzzleTwitchCode::" + tpCode)
		_ = c.WriteMessage(websocket.TextMessage, []byte("PuzzleTwitchCode::"+p.twitchId+"::"+tpCode))
	}

	return p
}

// genPuzzleCode generates a new puzzle code
// run this inside the lock
func (r *RemoteMath) genPuzzleCode() string {
	var c string
	for {
		c = makeId(r.rId, 6, idBytes)
		if _, exists := r.puzzles[c]; !exists {
			break
		}
	}
	return c
}

func formatFruits() {

}
