package main

import (
	"flag"
	remoteMath "github.com/MrMelon54/ktanemod-remote-math-server"
)

var addr string
var logDir string
var debugPuzzle bool

func main() {
	flag.StringVar(&addr, "addr", "localhost:8080", "service address")
	flag.StringVar(&logDir, "logs", "logs/", "log storage directory")
	flag.BoolVar(&debugPuzzle, "d", false, "enable to show puzzle debug logs")
	flag.Parse()

	s := &remoteMath.Server{Listen: addr, LogDir: logDir, DebugPuzzle: debugPuzzle}
	s.Run()
}
