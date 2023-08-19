package ktanemod_remote_math_server

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"testing"
)

var (
	fruits1 = [8]int{1, 3, 4, 1, 0, 3, 5, 2}
	fruits2 = [8]int{1, 3, 1, 1, 0, 3, 5, 2} // top fruit match
	fruits3 = [8]int{1, 3, 4, 1, 0, 3, 0, 3} // both expert fruit match

	cText1 = [2]int{0, 1}
	cText2 = [2]int{0, 0}
)

type testStructCheckSolution struct {
	out        bool
	a, b, c, d string
	batteries  int
	ports      int
	fruits     [8]int
	cText      [2]int
}

func (t testStructCheckSolution) Parsed() []string {
	return []string{t.Packet(), t.a, t.b, t.c, t.d}
}

func (t testStructCheckSolution) Packet() string {
	return fmt.Sprintf("PuzzleSolution::%s::%s::%s::%s", t.a, t.b, t.c, t.d)
}

func (t testStructCheckSolution) String() string {
	return fmt.Sprintf("%v - %s, %s, %s, %s", t.out, t.a, t.b, t.c, t.d)
}

var testCheckSolution = []testStructCheckSolution{
	// test fruits values
	{true, "2", "12", "14+91*5=469", "0", 2, 3, fruits1, cText1},
	{true, "9", "12", "21+42*5=231", "1", 2, 3, fruits2, cText1},
	{true, "11", "13", "24+91*5=479", "0", 2, 3, fruits3, cText1},

	// test cText2 same values
	{true, "11", "13", "24+91*5=479", "0", 2, 3, fruits3, cText2},

	// test batteries
	{true, "2", "12", "14+91*5=469", "0", 2, 3, fruits1, cText1},
	{true, "2", "20", "22+91*5=477", "0", 0, 3, fruits1, cText1},
	{true, "2", "24", "26+91*5=481", "0", 5, 3, fruits1, cText1},
	{true, "2", "20", "22+91-5=108", "0", 6, 3, fruits1, cText1},

	// test ports
	{true, "2", "12", "14+91*5=469", "0", 2, 3, fruits1, cText1},
	{true, "2", "12", "14+91*5=469", "0", 2, 0, fruits1, cText1},
	{true, "1", "12", "13+91*5=468", "0", 2, 7, fruits1, cText1},
}

func TestPuzzle_CheckSolution(t *testing.T) {
	for i, row := range testCheckSolution {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			logRaw := new(bytes.Buffer)
			p := &Puzzle{
				logRaw:    logRaw,
				log:       log.New(logRaw, "", 0),
				batteries: row.batteries,
				ports:     row.ports,
				fruits:    row.fruits,
				cText:     row.cText,
			}
			a := p.CheckSolution(row.Parsed())
			if row.out != a {
				s := bufio.NewScanner(logRaw)
				for s.Scan() {
					t.Log(s.Text())
				}
				t.Errorf("Solution should be valid")
			}
		})
	}
}
