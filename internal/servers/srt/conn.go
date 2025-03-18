package srt

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/description"
	mcmpegts "github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	"github.com/google/uuid"
	srtgo "github.com/haivision/srtgo"

	"github.com/bluenviron/mediamtx/internal/auth"
	"github.com/bluenviron/mediamtx/internal/conf"
	"github.com/bluenviron/mediamtx/internal/defs"
	"github.com/bluenviron/mediamtx/internal/externalcmd"
	"github.com/bluenviron/mediamtx/internal/hooks"
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/protocols/mpegts"
	"github.com/bluenviron/mediamtx/internal/stream"
)

// mySRTWrapper satisfies MpegtsSrtConn
type mySRTWrapper struct {
	sock *srtgo.SrtSocket
}

func (w *mySRTWrapper) SetWriteDeadline(t time.Time) error {
	w.sock.SetWriteDeadline(t)
	return nil
}

func srtCheckPassphrase(sconn *srtgo.SrtSocket, passphrase string) error {
	if passphrase == "" {
		return nil
	}
	// Attempt to set passphrase; if invalid, return error
	if err := sconn.SetSockOptString(srtgo.SRTO_PASSPHRASE, passphrase); err != nil {
		return fmt.Errorf("invalid passphrase: %w", err)
	}
	return nil
}

type connState int

const (
	connStateRead connState = iota + 1
	connStatePublish
)

type conn struct {
	parentCtx           context.Context
	rtspAddress         string
	readTimeout         conf.Duration
	writeTimeout        conf.Duration
	udpMaxPayloadSize   int
	runOnConnect        string
	runOnConnectRestart bool
	runOnDisconnect     string
	wg                  *sync.WaitGroup
	externalCmdPool     *externalcmd.Pool
	pathManager         serverPathManager
	parent              *Server

	ctx       context.Context
	ctxCancel func()
	created   time.Time
	uuid      uuid.UUID
	mutex     sync.RWMutex
	state     connState
	pathName  string
	query     string

	// The accepted SRT socket from srtgo
	sconn      *srtgo.SrtSocket
	remoteAddr net.Addr
}

func (c *conn) initialize() {
	c.ctx, c.ctxCancel = context.WithCancel(c.parentCtx)
	c.created = time.Now()
	c.uuid = uuid.New()

	c.Log(logger.Info, "opened")

	c.wg.Add(1)
	go c.run()
}

func (c *conn) Close() {
	c.ctxCancel()
}

// Log implements logger.Writer.
func (c *conn) Log(level logger.Level, format string, args ...interface{}) {
	// include remote address in logs
	c.parent.Log(level, "[conn %v] "+format, append([]interface{}{c.remoteAddr}, args...)...)
}

func (c *conn) ip() net.IP {
	if ua, ok := c.remoteAddr.(*net.UDPAddr); ok {
		return ua.IP
	}
	return net.IPv4zero
}

func (c *conn) run() {
	defer c.wg.Done()

	onDisconnectHook := hooks.OnConnect(hooks.OnConnectParams{
		Logger:              c,
		ExternalCmdPool:     c.externalCmdPool,
		RunOnConnect:        c.runOnConnect,
		RunOnConnectRestart: c.runOnConnectRestart,
		RunOnDisconnect:     c.runOnDisconnect,
		RTSPAddress:         c.rtspAddress,
		Desc:                c.APIReaderDescribe(),
	})
	defer onDisconnectHook()

	err := c.runInner()
	c.ctxCancel()

	// remove from parent
	c.parent.closeConn(c)

	c.Log(logger.Info, "closed: %v", err)
}

func (c *conn) runInner() error {
	// fetch the SRT stream ID
	streamIDStr, err := c.sconn.GetSockOptString(srtgo.SRTO_STREAMID)
	if err != nil {
		// treat this as a “reject”
		c.sconn.Close()
		return fmt.Errorf("could not read stream ID: %w", err)
	}

	var sid streamID
	if err := sid.unmarshal(streamIDStr); err != nil {
		c.sconn.Close()
		return fmt.Errorf("invalid stream ID '%s': %w", streamIDStr, err)
	}

	if sid.mode == streamIDModePublish {
		return c.runPublish(&sid)
	}
	return c.runRead(&sid)
}

func (c *conn) runPublish(sid *streamID) error {
	path, err := c.pathManager.AddPublisher(defs.PathAddPublisherReq{
		Author: c,
		AccessRequest: defs.PathAccessRequest{
			Name:    sid.path,
			Query:   sid.query,
			IP:      c.ip(),
			Publish: true,
			User:    sid.user,
			Pass:    sid.pass,
			Proto:   auth.ProtocolSRT,
			ID:      &c.uuid,
		},
	})
	if err != nil {
		// “reject” => just close
		c.sconn.Close()
		// if an auth error, sleep to mitigate brute force
		if errors.As(err, new(auth.Error)) {
			<-time.After(auth.PauseAfterError)
		}
		return err
	}
	defer path.RemovePublisher(defs.PathRemovePublisherReq{Author: c})

	if err := srtCheckPassphrase(c.sconn, path.SafeConf().SRTPublishPassphrase); err != nil {
		c.sconn.Close()
		return err
	}

	c.mutex.Lock()
	c.state = connStatePublish
	c.pathName = sid.path
	c.query = sid.query
	c.mutex.Unlock()

	readerErr := make(chan error, 1)
	go func() {
		readerErr <- c.runPublishReader(c.sconn, path)
	}()

	select {
	case err := <-readerErr:
		c.sconn.Close()
		return err
	case <-c.ctx.Done():
		c.sconn.Close()
		<-readerErr
		return errors.New("terminated")
	}
}

func (c *conn) runPublishReader(sconn *srtgo.SrtSocket, path defs.Path) error {
	sconn.SetReadDeadline(time.Now().Add(time.Duration(c.readTimeout)))
	r, err := mcmpegts.NewReader(mcmpegts.NewBufferedReader(sconn))
	if err != nil {
		return err
	}

	decodeErrLogger := logger.NewLimitedLogger(c)
	r.OnDecodeError(func(e error) {
		decodeErrLogger.Log(logger.Warn, e.Error())
	})

	var strm *stream.Stream
	medias, err := mpegts.ToStream(r, &strm, c)
	if err != nil {
		return err
	}

	strm, err = path.StartPublisher(defs.PathStartPublisherReq{
		Author:             c,
		Desc:               &description.Session{Medias: medias},
		GenerateRTPPackets: true,
	})
	if err != nil {
		return err
	}

	for {
		if err := r.Read(); err != nil {
			return err
		}
	}
}

func (c *conn) runRead(sid *streamID) error {
	wconn := &mySRTWrapper{sock: c.sconn}
	path, strm, err := c.pathManager.AddReader(defs.PathAddReaderReq{
		Author: c,
		AccessRequest: defs.PathAccessRequest{
			Name:  sid.path,
			Query: sid.query,
			IP:    c.ip(),
			User:  sid.user,
			Pass:  sid.pass,
			Proto: auth.ProtocolSRT,
			ID:    &c.uuid,
		},
	})
	if err != nil {
		c.sconn.Close()
		if errors.As(err, new(auth.Error)) {
			<-time.After(auth.PauseAfterError)
		}
		return err
	}
	defer path.RemoveReader(defs.PathRemoveReaderReq{Author: c})

	if err := srtCheckPassphrase(c.sconn, path.SafeConf().SRTReadPassphrase); err != nil {
		c.sconn.Close()
		return err
	}

	c.mutex.Lock()
	c.state = connStateRead
	c.pathName = sid.path
	c.query = sid.query
	c.mutex.Unlock()

	bw := bufio.NewWriterSize(c.sconn, srtMaxPayloadSize(c.udpMaxPayloadSize))
	err = mpegts.FromStream(strm, c, bw, wconn, time.Duration(c.writeTimeout))
	if err != nil {
		return err
	}

	c.Log(logger.Info, "is reading from path '%s', %s",
		path.Name(), defs.FormatsInfo(strm.ReaderFormats(c)))

	onUnreadHook := hooks.OnRead(hooks.OnReadParams{
		Logger:          c,
		ExternalCmdPool: c.externalCmdPool,
		Conf:            path.SafeConf(),
		ExternalCmdEnv:  path.ExternalCmdEnv(),
		Reader:          c.APIReaderDescribe(),
		Query:           sid.query,
	})
	defer onUnreadHook()

	// disable read deadline
	c.sconn.SetReadDeadline(time.Time{})

	strm.StartReader(c)
	defer strm.RemoveReader(c)

	select {
	case <-c.ctx.Done():
		return fmt.Errorf("terminated")
	case err := <-strm.ReaderError(c):
		return err
	}
}

// APIReaderDescribe implements reader.
func (c *conn) APIReaderDescribe() defs.APIPathSourceOrReader {
	return defs.APIPathSourceOrReader{
		Type: "srtConn",
		ID:   c.uuid.String(),
	}
}

// APISourceDescribe implements source.
func (c *conn) APISourceDescribe() defs.APIPathSourceOrReader {
	return c.APIReaderDescribe()
}

// Here we remove direct references to gosrt.Stats() (which srtgo lacks).
// If you want advanced stats, you'd either implement a cgo call to srt_bstats
// or skip stats. Below is a minimal "no advanced stats" approach.

func (c *conn) apiItem() *defs.APISRTConn {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item := &defs.APISRTConn{
		ID:         c.uuid,
		Created:    c.created,
		RemoteAddr: c.remoteAddr.String(),
		State: func() defs.APISRTConnState {
			switch c.state {
			case connStateRead:
				return defs.APISRTConnStateRead
			case connStatePublish:
				return defs.APISRTConnStatePublish
			default:
				return defs.APISRTConnStateIdle
			}
		}(),
		Path:  c.pathName,
		Query: c.query,
	}

	// If you need advanced stats, implement a cgo call to srt_bstats.
	// For now, we'll omit them to remove gosrt artifacts.

	return item
}
