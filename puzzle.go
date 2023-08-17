package ktanemod_remote_math_server

import (
	"bytes"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	regPuzzleSolution           = regexp.MustCompile("^PuzzleSolution::([0-9]+)::([0-9]+)::([0-9−+÷×=]+)::([0-5])$")
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

// these constants are to prevent confusion with other symbols in the source code
const (
	specialDash  = "\xe2\x88\x92"
	specialTimes = "\xc3\x97"
)

type Puzzle struct {
	code        string
	date        time.Time
	logRaw      *bytes.Buffer
	log         *log.Logger
	modConn     *websocket.Conn
	webConnLock *sync.RWMutex
	webConns    []*WebConn
	twitchPlays bool
	twitchId    string
	killLock    *sync.RWMutex
	killed      bool

	batteries int
	ports     int
	fruits    [8]int
	cText     [2]int
}

func NewPuzzle(conn *websocket.Conn) *Puzzle {
	logRaw := new(bytes.Buffer)
	return &Puzzle{
		date:        time.Now(),
		logRaw:      logRaw,
		log:         log.New(logRaw, "", 0),
		modConn:     conn,
		webConnLock: new(sync.RWMutex),
		webConns:    make([]*WebConn, 0),
		killLock:    new(sync.RWMutex),
	}
}

type WebConn struct {
	conn   *websocket.Conn
	tpDone bool
	tpCode string
}

func (p *Puzzle) checkSolution(sln []string) bool {
	slnn := [4]int{
		mustParseInt(sln[1]),
		mustParseInt(sln[2]),
		0,
		mustParseInt(sln[4]),
	}
	// Solution :: [1] Left fruit :: [2] Right fruit :: [3] Display content :: [4] Status light colour
	// Fruit numbers
	/* f1 = defuser's top
	 * f2 = defuser's right
	 * f3 = expert's left
	 * f4 = expert's right
	 */

	f1 := fruitNumbers[p.fruits[0]][p.fruits[2]]
	f2 := fruitNumbers[p.fruits[1]][p.fruits[3]]
	f3 := fruitNumbers[p.fruits[4]][p.fruits[6]]
	f4 := fruitNumbers[p.fruits[5]][p.fruits[7]]

	_ = f3 // I used the variable now lol

	// Step 1
	s1a1 := 0
	if p.fruits[0] == p.fruits[2] {
		s1a1 = 21
	}
	s1a := f1*13 + s1a1 - p.ports
	s1b := int(math.Abs(math.Floor(float64(s1a) / float64(f4))))
	s1c := s1b % 20

	// Step 2
	s2a1 := 0
	if p.fruits[4] == p.fruits[6] && p.fruits[5] == p.fruits[7] {
		s2a1 = 54
	}
	s2a := f4*f2 - s2a1
	var s2b int
	if p.batteries != 0 {
		s2b = int(math.Abs(math.Floor(float64(s2a) / float64(p.batteries))))
	} else {
		s2b = int(math.Abs(math.Floor(float64(s2a))))
	}
	s2c := (s2b % 20) + 5

	// Step 3
	var s3a string
	var s3b int
	if p.batteries > 5 {
		s3a = fmt.Sprintf("%d+%d%s%d=", s1c+s2c, f1, specialDash, f2)
		s3b = s1c + s2c + (f1 - f2)
	} else {
		s3a = fmt.Sprintf("%d+%d%s%d=", s1c+s2c, f1, specialTimes, f2)
		s3b = s1c + s2c + (f1 * f2)
	}
	s3c := s3a + strings.ReplaceAll(strconv.Itoa(s3b), "-", specialDash)

	// Step 4
	s4a := slnn[3] == p.cText[0] || slnn[3] == p.cText[1]

	p.log.Println("Correct Answers:")
	p.log.Printf("  Step 1: %d\n", s1c)
	p.log.Printf("  Step 2: %d\n", s2c)
	p.log.Printf("  Step 3: %s\n", s3c)
	p.log.Printf("  Step 4: %d or %d\n", p.cText[0], p.cText[1])
	p.log.Println("Your Answers:")
	p.log.Printf("  Step 1: %d\n", sln[1])
	p.log.Printf("  Step 2: %d\n", sln[2])
	p.log.Printf("  Step 3: %d\n", sln[3])
	p.log.Printf("  Step 4: %d\n", sln[4])

	c1 := s1c == slnn[0]
	c2 := s2c == slnn[1]
	c3 := s3c == sln[3]
	c4 := s4a

	p.log.Println("Checking Answers:")
	p.log.Printf("  Step 1: %v\n", c1)
	p.log.Printf("  Step 2: %v\n", c2)
	p.log.Printf("  Step 3: %v\n", c3)
	p.log.Printf("  Step 4: %v\n", c4)

	return c1 && c2 && c3 && c4
}

func (p *Puzzle) checkKilled() bool {
	p.killLock.RLock()
	defer p.killLock.RUnlock()
	return p.killed
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
	}
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
		if p.checkSolution(submatch) {
			p.log.Println("Correct solution")
			p.SendMod("PuzzleLog::CorrectSolution")
			p.log.Println("Sending solve")
			p.SendMod("PuzzleComplete")
			p.SendWebConns("PuzzleComplete")

			// this triggers Kill later
			_ = p.modConn.Close()
		}
		return
	}

}

func (p *Puzzle) RemoveWebConn(c *websocket.Conn) {
	p.webConnLock.Lock()
	for i := range p.webConns {
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
	p.killLock.Lock()
	p.killed = true
	p.killLock.Unlock()
	_ = p.modConn.Close()
	p.webConnLock.RLock()
	for _, i := range p.webConns {
		_ = i.conn.Close()
	}
	p.webConnLock.RUnlock()
}

func mustParseInt(s string) int {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0
	}
	return int(n)
}
