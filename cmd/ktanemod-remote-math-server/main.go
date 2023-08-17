package main

import (
	"flag"
	remoteMath "github.com/MrMelon54/ktanemod-remote-math-server"
)

var addr string

func main() {
	flag.StringVar(&addr, "addr", "localhost:8080", "service address")
	flag.Parse()

	s := &remoteMath.Server{Listen: addr}
	s.Run()
}
