package ktanemod_remote_math_server

import (
	"bytes"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"math"
	"os"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	regPuzzleSolution           = regexp.MustCompile("^PuzzleSolution::([0-9]+)::([0-9]+)::([0-9-+/*=]+)::([0-5])$")
	regPuzzleTwitchPlaysMode    = regexp.MustCompile("^PuzzleTwitchPlaysMode::([0-9]+)$")
	regPuzzleActivateTwitchCode = regexp.MustCompile("^PuzzleActivateTwitchCode::([0-9]{3})$")
	regPuzzleFruits             = regexp.MustCompile("^PuzzleFruits::([0-5])::([0-5])::([0-5])::([0-5])::([0-5])::([0-5])::([0-5])::([0-5])$")
	regBombDetails              = regexp.MustCompile("^BombDetails::([0-9]+)::([0-9]+)$")
)

var (
	fruitNames   = []string{"Apple", "Melon", "Orange", "Pear", "Pineapple", "Strawberry"}
	fruitNumbers = [][]int{
		{88, 1, 48, 75, 31, 8},
		{84, 42, 62, 21, 91, 17},
		{56, 29, 12, 53, 11, 81},
		{32, 5, 19, 38, 25, 64},
		{44, 61, 20, 92, 13, 4},
		{34, 50, 87, 22, 54, 19},
	}
)

type Puzzle struct {
	code        string
	date        time.Time
	saveLog     *atomic.Bool
	logRaw      *bytes.Buffer
	log         *log.Logger
	modConn     *websocket.Conn
	webConnLock *sync.RWMutex
	webConns    []*WebConn
	twitchPlays bool
	twitchId    string
	killed      *atomic.Bool

	batteries int
	ports     int
	fruits    [8]int
	cText     [2]int
}

func NewPuzzle(conn *websocket.Conn, debug bool) *Puzzle {
	logRaw := new(bytes.Buffer)
	var logOut io.Writer
	if debug {
		logOut = io.MultiWriter(logRaw, log.New(os.Stderr, "DebugPuzzle", 0).Writer())
	} else {
		logOut = logRaw
	}
	return &Puzzle{
		date:        time.Now(),
		saveLog:     new(atomic.Bool),
		logRaw:      logRaw,
		log:         log.New(logOut, "", 0),
		modConn:     conn,
		webConnLock: new(sync.RWMutex),
		webConns:    make([]*WebConn, 0),
		killed:      new(atomic.Bool),
	}
}

type WebConn struct {
	conn   *websocket.Conn
	tpDone bool
	tpCode string
}

func (p *Puzzle) CheckSolution(sln []string) bool {
	sln1 := mustParseInt(sln[1])
	sln2 := mustParseInt(sln[2])
	sln3 := sln[3]
	sln4 := mustParseInt(sln[4])
	// Solution :: [1] Left fruit :: [2] Right fruit :: [3] Display content :: [4] Status light colour
	// Fruit numbers
	/* f1 = defuser's top
	 * f2 = defuser's right
	 * f3 = expert's left
	 * f4 = expert's right
	 */

	f1 := fruitNumbers[p.fruits[0]][p.fruits[2]] // top defuser
	f2 := fruitNumbers[p.fruits[1]][p.fruits[3]] // right defuser
	f3 := fruitNumbers[p.fruits[4]][p.fruits[6]] // left expert
	f4 := fruitNumbers[p.fruits[5]][p.fruits[7]] // right expert

	_ = f3 // I used the variable now lol

	// Step 1
	s1f := float64(f1 * 13)
	if p.fruits[0] == p.fruits[2] {
		s1f += 21
	}
	s1f -= float64(p.ports)
	s1f /= float64(f4)
	s1int := int(math.Abs(s1f))
	s1int %= 20

	// Step 2
	s2f := float64(f4 * f2)
	if p.fruits[4] == p.fruits[6] && p.fruits[5] == p.fruits[7] {
		s2f -= 54
	}
	if p.batteries != 0 {
		s2f /= float64(p.batteries)
	}
	s2int := int(math.Abs(s2f))
	s2int %= 20
	s2int += 5

	// Step 3
	s3a := s1int + s2int
	s3b := f1
	s3c := f2
	var s3d int
	var s3str string
	if p.batteries > 5 {
		s3d = s3a + s3b - s3c
		s3str = fmt.Sprintf("%d+%d-%d=%d", s3a, s3b, s3c, s3d)
	} else {
		s3d = s3a + s3b*s3c
		s3str = fmt.Sprintf("%d+%d*%d=%d", s3a, s3b, s3c, s3d)
	}

	p.log.Println("Correct Answers:")
	p.log.Printf("  Step 1: %d\n", s1int)
	p.log.Printf("  Step 2: %d\n", s2int)
	p.log.Printf("  Step 3: %s\n", s3str)
	p.log.Printf("  Step 4: %d or %d\n", p.cText[0], p.cText[1])
	p.log.Println("Your Answers:")
	p.log.Printf("  Step 1: %d\n", sln1)
	p.log.Printf("  Step 2: %d\n", sln2)
	p.log.Printf("  Step 3: %s\n", sln3)
	p.log.Printf("  Step 4: %d\n", sln4)

	c1 := s1int == sln1
	c2 := s2int == sln2
	c3 := s3str == sln3
	c4 := sln4 == p.cText[0] || sln4 == p.cText[1]

	p.log.Println("Checking Answers:")
	p.log.Printf("  Step 1: %v\n", c1)
	p.log.Printf("  Step 2: %v\n", c2)
	p.log.Printf("  Step 3: %v\n", c3)
	p.log.Printf("  Step 4: %v\n", c4)

	return c1 && c2 && c3 && c4
}

func (p *Puzzle) checkKilled() bool {
	return p.killed.Load()
}

func (p *Puzzle) SendMod(s string) {
	if p.checkKilled() {
		return
	}
	_ = p.modConn.WriteMessage(websocket.TextMessage, []byte(s))
}

func (p *Puzzle) RecvMod(s string) {
	submatch := regPuzzleTwitchPlaysMode.FindStringSubmatch(s)
	if submatch != nil {
		p.twitchPlays = true
		p.twitchId = submatch[1]
		return
	}
	submatch = regPuzzleActivateTwitchCode.FindStringSubmatch(s)
	if submatch != nil {
		p.webConnLock.Lock()
		for _, i := range p.webConns {
			if i.tpCode == submatch[1] {
				i.tpDone = true
				_ = i.conn.WriteMessage(websocket.TextMessage, []byte("PuzzleActivateTwitchPlays"))
				break
			}
		}
		p.webConnLock.Unlock()
		return
	}
	submatch = regPuzzleFruits.FindStringSubmatch(s)
	if submatch != nil {
		p.fruits = [8]int{
			mustParseInt(submatch[1]),
			mustParseInt(submatch[2]),
			mustParseInt(submatch[3]),
			mustParseInt(submatch[4]),
			mustParseInt(submatch[5]),
			mustParseInt(submatch[6]),
			mustParseInt(submatch[7]),
			mustParseInt(submatch[8]),
		}
		f := [8]string{}
		for i := range p.fruits {
			f[i] = fruitNames[p.fruits[i]]
		}
		f1 := fruitNumbers[p.fruits[0]][p.fruits[2]]
		f2 := fruitNumbers[p.fruits[1]][p.fruits[3]]
		f3 := fruitNumbers[p.fruits[4]][p.fruits[6]]
		f4 := fruitNumbers[p.fruits[5]][p.fruits[7]]
		p.log.Printf(`Fruits: +---------------+------------+------------+--------+
        | Position      | Image      | Text       | Number |
        | Defuser Top   | %-10s | %-10s | %6d |
        | Defuser Right | %-10s | %-10s | %6d |
        | Expert Left   | %-10s | %-10s | %6d |
        | Expert Right  | %-10s | %-10s | %6d |
        +---------------+------------+------------+--------+
`, f[0], f[2], f1, f[1], f[3], f2, f[4], f[6], f3, f[5], f[7], f4)
		return
	}
	submatch = regBombDetails.FindStringSubmatch(s)
	if submatch != nil {
		p.batteries = mustParseInt(submatch[1])
		p.ports = mustParseInt(submatch[2])
		p.log.Printf("Batteries: %d\n", p.batteries)
		p.log.Printf("Ports: %d\n", p.ports)
		return
	}

	log.Printf("Unknown packet '%s' from module\n", s)
}

func (p *Puzzle) SendWebConns(s string) {
	if p.checkKilled() {
		return
	}
	p.webConnLock.RLock()
	for _, i := range p.webConns {
		_ = i.conn.WriteMessage(websocket.TextMessage, []byte(s))
	}
	p.webConnLock.RUnlock()
}

func (p *Puzzle) RecvWebConn(s string) {
	submatch := regPuzzleSolution.FindStringSubmatch(s)
	if submatch != nil {
		// log will only save after first solution check
		p.saveLog.Store(true)

		if p.CheckSolution(submatch) {
			p.log.Println("Correct solution")
			p.SendMod("PuzzleLog::CorrectSolution")
			p.log.Println("Sending solve")
			p.SendMod("PuzzleComplete")
			p.SendWebConns("PuzzleComplete")

			go func() {
				// force close module connection after 5 seconds
				<-time.After(5 * time.Second)
				_ = p.modConn.Close()
			}()
		}
		return
	}

	log.Printf("Unknown packet '%s' from web client\n", s)
}

func (p *Puzzle) RemoveWebConn(c *websocket.Conn) {
	p.webConnLock.Lock()
	for i := range p.webConns {
		if i >= len(p.webConns) {
			break
		}
		if p.webConns[i].conn == c {
			l := len(p.webConns)
			p.webConns[i] = p.webConns[l-1]
			p.webConns = p.webConns[:l-1]
		}
	}
	p.webConnLock.Unlock()
}

func (p *Puzzle) Kill() {
	if p.checkKilled() {
		return
	}
	p.killed.Store(true)
	_ = p.modConn.Close()
	p.webConnLock.RLock()
	for _, i := range p.webConns {
		_ = i.conn.Close()
	}
	p.webConnLock.RUnlock()
}

func (p *Puzzle) TPCodeExists(code string) bool {
	for _, i := range p.webConns {
		if i.tpCode == code {
			// code already exists
			return true
		}
	}
	// code does not exist
	return false
}

func mustParseInt(s string) int {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0
	}
	return int(n)
}
