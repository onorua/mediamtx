package srt

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	srtgo "github.com/haivision/srtgo"

	"github.com/bluenviron/mediamtx/internal/conf"
	"github.com/bluenviron/mediamtx/internal/defs"
	"github.com/bluenviron/mediamtx/internal/externalcmd"
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/stream"
)

// ErrConnNotFound is returned when a connection is not found.
var ErrConnNotFound = errors.New("connection not found")

// srtMaxPayloadSize is same as before
func srtMaxPayloadSize(u int) int {
	return ((u - 16) / 188) * 188 // 16 = SRT header, 188 = MPEG-TS packet
}

// Server is an SRT server that accepts multiple connections concurrently.
type Server struct {
	Address             string
	RTSPAddress         string
	ReadTimeout         conf.Duration
	WriteTimeout        conf.Duration
	UDPMaxPayloadSize   int
	RunOnConnect        string
	RunOnConnectRestart bool
	RunOnDisconnect     string
	ExternalCmdPool     *externalcmd.Pool
	PathManager         serverPathManager
	Parent              serverParent // logger + parent interface

	ctx       context.Context
	ctxCancel func()
	wg        sync.WaitGroup

	// The new "native" SRT listener
	ln *listener

	// concurrency
	mutex sync.Mutex // Add this mutex to protect the map
	conns map[*conn]struct{}

	// channels used by run() loop
	chAcceptErr    chan error
	chCloseConn    chan *conn
	chAPIConnsList chan serverAPIConnsListReq
	chAPIConnsGet  chan serverAPIConnsGetReq
	chAPIConnsKick chan serverAPIConnsKickReq
}

// serverPathManager is used by the conn logic (unchanged).
type serverPathManager interface {
	AddPublisher(req defs.PathAddPublisherReq) (defs.Path, error)
	AddReader(req defs.PathAddReaderReq) (defs.Path, *stream.Stream, error)
}

// serverParent is a minimal interface for logging (unchanged).
type serverParent interface {
	logger.Writer
	GetLogLevel() logger.Level
	SrtStatsLoggingEnabled() bool
	GetSRTStatsInterval() time.Duration
}

// serverAPIConnsListReq, serverAPIConnsGetReq, serverAPIConnsKickReq are unchanged:
type serverAPIConnsListRes struct {
	data *defs.APISRTConnList
	err  error
}
type serverAPIConnsListReq struct {
	res chan serverAPIConnsListRes
}

type serverAPIConnsGetRes struct {
	data *defs.APISRTConn
	err  error
}
type serverAPIConnsGetReq struct {
	uuid uuid.UUID
	res  chan serverAPIConnsGetRes
}

type serverAPIConnsKickRes struct {
	err error
}
type serverAPIConnsKickReq struct {
	uuid uuid.UUID
	res  chan serverAPIConnsKickRes
}

// logLevelToSRTLevel converts a logger.Level to srtgo.SrtLogLevel
func logLevelToSRTLevel(level logger.Level) srtgo.SrtLogLevel {
	switch level {
	case logger.Error:
		return srtgo.SrtLogLevelErr
	case logger.Warn:
		return srtgo.SrtLogLevelWarning
	case logger.Info:
		return srtgo.SrtLogLevelInfo
	case logger.Debug:
		return srtgo.SrtLogLevelDebug
	default:
		return srtgo.SrtLogLevelInfo
	}
}

// srtLevelToLogLevel converts a srtgo.SrtLogLevel to logger.Level
func srtLevelToLogLevel(level srtgo.SrtLogLevel) logger.Level {
	switch level {
	case srtgo.SrtLogLevelCrit, srtgo.SrtLogLevelErr:
		return logger.Error
	case srtgo.SrtLogLevelWarning:
		return logger.Warn
	case srtgo.SrtLogLevelNotice, srtgo.SrtLogLevelInfo:
		return logger.Info
	case srtgo.SrtLogLevelDebug:
		return logger.Debug
	default:
		return logger.Info
	}
}

// Initialize sets up the SRT listener and concurrency channels.
func (s *Server) Initialize() error {
	s.ctx, s.ctxCancel = context.WithCancel(context.Background())
	s.conns = make(map[*conn]struct{})

	srtLogLevel := logLevelToSRTLevel(s.Parent.GetLogLevel())
	srtgo.SrtSetLogLevel(srtLogLevel)

	srtgo.SrtSetLogHandler(func(level srtgo.SrtLogLevel, file string, line int, area, message string) {
		logLevel := srtLevelToLogLevel(level)
		s.Log(logLevel, "[SRT-LIB] %s: %s", area, message)
	})

	// create channels
	s.chAcceptErr = make(chan error)
	s.chCloseConn = make(chan *conn)
	s.chAPIConnsList = make(chan serverAPIConnsListReq)
	s.chAPIConnsGet = make(chan serverAPIConnsGetReq)
	s.chAPIConnsKick = make(chan serverAPIConnsKickReq)

	// parse host/port from s.Address
	host, portStr, err := net.SplitHostPort(s.Address)
	if err != nil {
		return fmt.Errorf("invalid address '%s': %w", s.Address, err)
	}
	// minimal port parse
	p, err := net.LookupPort("udp", portStr)
	if err != nil {
		return fmt.Errorf("invalid port in address '%s': %w", s.Address, err)
	}

	// build srtgo options
	options := map[string]string{
		"transtype": "live", // or "file"
		// If you want to set other SRT options, do so here
	}
	// create our new srt listener
	ln, err := newListener(host, uint16(p), options)
	if err != nil {
		return err
	}
	s.ln = ln
	s.ln.parent = s // link back for concurrency
	s.ln.wg = &s.wg

	// start the listener goroutine
	s.ln.initialize()

	s.Log(logger.Info, "listener opened on %s (SRT)", s.Address)

	// spawn the main server loop
	s.wg.Add(1)
	go s.run()

	// start logging SRT stats
	s.logSRTStats()

	return nil
}

// run is the main event loop for the server.
func (s *Server) run() {
	defer s.wg.Done()

outer:
	for {
		select {
		case err := <-s.chAcceptErr:
			s.Log(logger.Error, "%s", err)
			break outer

		case c := <-s.chCloseConn:
			// a conn has closed
			s.mutex.Lock()
			delete(s.conns, c)
			s.mutex.Unlock()

		case req := <-s.chAPIConnsList:
			data := &defs.APISRTConnList{Items: []*defs.APISRTConn{}}
			for c := range s.conns {
				data.Items = append(data.Items, c.apiItem())
			}
			sort.Slice(data.Items, func(i, j int) bool {
				return data.Items[i].Created.Before(data.Items[j].Created)
			})
			req.res <- serverAPIConnsListRes{data: data}

		case req := <-s.chAPIConnsGet:
			c := s.findConnByUUID(req.uuid)
			if c == nil {
				req.res <- serverAPIConnsGetRes{err: ErrConnNotFound}
				continue
			}
			req.res <- serverAPIConnsGetRes{data: c.apiItem()}

		case req := <-s.chAPIConnsKick:
			c := s.findConnByUUID(req.uuid)
			if c == nil {
				req.res <- serverAPIConnsKickRes{err: ErrConnNotFound}
				continue
			}
			s.mutex.Lock()
			delete(s.conns, c)
			s.mutex.Unlock()
			c.Close()
			req.res <- serverAPIConnsKickRes{}

		case <-s.ctx.Done():
			break outer
		}
	}

	s.ctxCancel()
	s.ln.Close()
}

// newConn is called by the listener when a new SRT connection is accepted
func (s *Server) newConn(sock *srtgo.SrtSocket, remote net.Addr) {
	// create a new conn struct (see "conn.go")
	c := &conn{
		parentCtx:           s.ctx,
		rtspAddress:         s.RTSPAddress,
		readTimeout:         s.ReadTimeout,
		writeTimeout:        s.WriteTimeout,
		udpMaxPayloadSize:   s.UDPMaxPayloadSize,
		runOnConnect:        s.RunOnConnect,
		runOnConnectRestart: s.RunOnConnectRestart,
		runOnDisconnect:     s.RunOnDisconnect,
		wg:                  &s.wg,
		externalCmdPool:     s.ExternalCmdPool,
		pathManager:         s.PathManager,
		parent:              s,
		sconn:               sock,
		remoteAddr:          remote,
	}
	c.initialize()
	s.mutex.Lock()
	s.conns[c] = struct{}{}
	s.mutex.Unlock()
}

// acceptError is called by the listener if Accept() fails
func (s *Server) acceptError(err error) {
	select {
	case s.chAcceptErr <- err:
	case <-s.ctx.Done():
	}
}

// findConnByUUID returns a conn matching the provided UUID
func (s *Server) findConnByUUID(u uuid.UUID) *conn {
	for cx := range s.conns {
		if cx.uuid == u {
			return cx
		}
	}
	return nil
}

func (s *Server) closeConn(c *conn) {
	// if you track connections, remove c from the map, or send it over a channel
	select {
	case s.chCloseConn <- c:
	case <-s.ctx.Done():
	}
}

// Close stops everything
func (s *Server) Close() {
	s.Log(logger.Info, "listener is closing")
	s.ctxCancel()
	s.wg.Wait()
}

// APIConnsList, APIConnsGet, APIConnsKick are unchanged

func (s *Server) APIConnsList() (*defs.APISRTConnList, error) {
	req := serverAPIConnsListReq{res: make(chan serverAPIConnsListRes)}
	select {
	case s.chAPIConnsList <- req:
		res := <-req.res
		return res.data, res.err
	case <-s.ctx.Done():
		return nil, fmt.Errorf("terminated")
	}
}

func (s *Server) APIConnsGet(u uuid.UUID) (*defs.APISRTConn, error) {
	req := serverAPIConnsGetReq{uuid: u, res: make(chan serverAPIConnsGetRes)}
	select {
	case s.chAPIConnsGet <- req:
		res := <-req.res
		return res.data, res.err
	case <-s.ctx.Done():
		return nil, fmt.Errorf("terminated")
	}
}

func (s *Server) APIConnsKick(u uuid.UUID) error {
	req := serverAPIConnsKickReq{uuid: u, res: make(chan serverAPIConnsKickRes)}
	select {
	case s.chAPIConnsKick <- req:
		res := <-req.res
		return res.err
	case <-s.ctx.Done():
		return fmt.Errorf("terminated")
	}
}

// Log delegates to the parent's logger
func (s *Server) Log(level logger.Level, format string, args ...interface{}) {
	s.Parent.Log(level, "[SRT] "+format, args...)
}

// Add a method to fetch and log SRT stats at configured intervals.
func (s *Server) logSRTStats() {
	// Check if SRT stats logging is enabled
	if !s.Parent.SrtStatsLoggingEnabled() {
		return
	}

	interval := s.Parent.GetSRTStatsInterval()
	if interval <= 0 {
		interval = 1 * time.Second
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(interval):
				s.mutex.Lock()
				for conn := range s.conns {
					stats, err := conn.sconn.Stats()
					if err != nil {
						s.Log(logger.Warn, "Error fetching stats: %v", err)
						continue
					}
					s.Log(logger.Info, "[%s] SRT Stats: %+v", conn.uuid, stats)
				}
				s.mutex.Unlock()
			}
		}
	}()
}
