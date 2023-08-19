package ktanemod_remote_math_server

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

type FakeSource struct {
	a []int64
	i int
}

func (f *FakeSource) Int63() int64 {
	if f.i >= len(f.a) {
		panic("end of fake source")
	}
	b := f.a[f.i]
	f.i++
	return b
}

func (f *FakeSource) Seed(a int64) {}

func TestMakeId(t *testing.T) {
	r := rand.New(&FakeSource{a: []int64{6692326730743115, 0, 929525840576556069, 0}})
	assert.Equal(t, "CADA", MakeId(r, 4, "ABCDEFGH"))
}
