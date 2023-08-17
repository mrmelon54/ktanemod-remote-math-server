package ktanemod_remote_math_server

import (
	"math/rand"
	"strings"
)

func makeId(r *rand.Rand, l int, chars string) string {
	var s strings.Builder
	s.Grow(l)
	for i := 0; i < l; i++ {
		s.WriteByte(chars[r.Intn(len(chars))])
	}
	return s.String()
}
