package srt

import (
	"sync"

	"github.com/haivision/srtgo"
	srt "github.com/bluenviron/mediamtx/internal/srtcompat"
)

type listener struct {
	ln     srt.Listener
	sock   *srtgo.SrtSocket  // Direct srtgo socket for performance
	wg     *sync.WaitGroup
	parent *Server
}

func (l *listener) initialize() {
	go l.run()
}

func (l *listener) run() {
	err := l.runInner()

	l.parent.acceptError(err)
}

func (l *listener) runInner() error {
	for {
		select {
		case <-l.parent.ctx.Done():
			// The server is shutting down, exit immediately
			return nil
		default:
			req, err := l.ln.Accept2()
			if err != nil {
				return err
			}

			l.parent.newConnRequest(req)
		}
	}
}
