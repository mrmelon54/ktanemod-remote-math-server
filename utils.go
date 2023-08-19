package ktanemod_remote_math_server

import (
	"math/rand"
	"strings"
)

func MakeId(r *rand.Rand, l int, chars string) string {
	var s strings.Builder
	s.Grow(l)
	for i := 0; i < l; i++ {
		b := r.Intn(len(chars))
		s.WriteByte(chars[b])
	}
	return s.String()
}
