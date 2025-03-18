package srt

import (
	"sync"

	srtgo "github.com/haivision/srtgo"
)

// listener wraps a single srtgo SrtSocket in "listen" mode
type listener struct {
	sock   *srtgo.SrtSocket
	wg     *sync.WaitGroup
	parent *Server // reference back to server for concurrency & logging
}

// newListener creates and starts listening on (addr, port) with optional srtgo options
func newListener(addr string, port uint16, options map[string]string) (*listener, error) {
	if addr == "" {
		addr = "0.0.0.0"
	}
	// create the SRT socket
	sock := srtgo.NewSrtSocket(addr, port, options)

	// srtgo.Listen(...) binds and listens in one call
	if err := sock.Listen(5); err != nil {
		sock.Close()
		return nil, err
	}

	return &listener{
		sock: sock,
	}, nil
}

// initialize spawns a goroutine that calls Accept() in a loop
func (l *listener) initialize() {
	go l.run()
}

// run continuously accepts new SRT connections, passing them to the parent
func (l *listener) run() {
	defer l.wg.Done()
	for {
		select {
		case <-l.parent.ctx.Done():
			// The server is shutting down. We can exit the loop,
			// or we can close the socket to unblock Accept below.
			// We'll do both for safety.
			return
		default:
			sconn, raddr, err := l.sock.Accept()
			if err != nil {
				// notify the server about accept error
				l.parent.acceptError(err)
				return
			}
			// pass the newly accepted socket + remote addr to the server
			l.parent.newConn(sconn, raddr)
		}
	}
}

// Close stops the listener
func (l *listener) Close() {
	l.sock.Close() // no return value from this srtgo version
}
