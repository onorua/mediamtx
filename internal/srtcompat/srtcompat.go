// Package srtcompat provides a gosrt-compatible API wrapper around srtgo
package srtcompat

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/haivision/srtgo"
)

// Removed old logging functions - now using SetupSRTLogging instead

// logLevelToSRTLevel converts a logger level to srtgo log level
func logLevelToSRTLevel(level int) srtgo.SrtLogLevel {
	switch level {
	case 0: // Error
		return srtgo.SrtLogLevelErr
	case 1: // Warn
		return srtgo.SrtLogLevelWarning
	case 2: // Info
		return srtgo.SrtLogLevelInfo
	case 3: // Debug
		return srtgo.SrtLogLevelDebug
	default:
		return srtgo.SrtLogLevelInfo
	}
}

// srtLevelToLogLevel converts srtgo log level to logger level
func srtLevelToLogLevel(level srtgo.SrtLogLevel) int {
	switch level {
	case srtgo.SrtLogLevelCrit, srtgo.SrtLogLevelErr:
		return 0 // Error
	case srtgo.SrtLogLevelWarning:
		return 1 // Warn
	case srtgo.SrtLogLevelNotice, srtgo.SrtLogLevelInfo:
		return 2 // Info
	case srtgo.SrtLogLevelDebug:
		return 3 // Debug
	default:
		return 2 // Info
	}
}

// InitSRT initializes the SRT library with logging
func InitSRT() {
	srtgo.InitSRT()
}

// SetupSRTLogging configures SRT logging integration
func SetupSRTLogging(logLevel int, logFunc func(level int, message string)) {
	// Set SRT log level
	srtLogLevel := logLevelToSRTLevel(logLevel)
	srtgo.SrtSetLogLevel(srtLogLevel)

	// Set up log handler
	srtgo.SrtSetLogHandler(func(level srtgo.SrtLogLevel, file string, line int, area, message string) {
		logLevel := srtLevelToLogLevel(level)
		logFunc(logLevel, fmt.Sprintf("[SRT] %s: %s", area, message))
	})
}

// CleanupSRT is no longer needed - the OS handles SRT cleanup when process exits
func CleanupSRT() {
	// No-op: SRT library cleanup is handled by the OS when the process exits
	// This prevents hanging during shutdown while maintaining API compatibility
}

// Rejection reason constants to match gosrt
const (
	REJ_PEER  = 1003
	REJ_CLOSE = 1002
)

// Config represents SRT configuration, compatible with gosrt.Config
type Config struct {
	ConnectionTimeout time.Duration
	PayloadSize       uint32
	Passphrase        string
	StreamID          string
}

// DefaultConfig returns a default SRT configuration
func DefaultConfig() *Config {
	return &Config{
		ConnectionTimeout: 3 * time.Second,
		PayloadSize:       1316,
	}
}

// UnmarshalURL parses an SRT URL and extracts connection parameters
func (c *Config) UnmarshalURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	if u.Scheme != "srt" {
		return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	// Extract query parameters
	query := u.Query()
	if streamid := query.Get("streamid"); streamid != "" {
		c.StreamID = streamid
	}
	if passphrase := query.Get("passphrase"); passphrase != "" {
		c.Passphrase = passphrase
	}

	// Return host:port
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "1935" // Default SRT port
	}

	return net.JoinHostPort(host, port), nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ConnectionTimeout <= 0 {
		return fmt.Errorf("invalid connection timeout")
	}
	if c.PayloadSize == 0 {
		return fmt.Errorf("invalid payload size")
	}
	return nil
}

// Listener represents an SRT listener, compatible with gosrt.Listener
type Listener struct {
	socket *srtgo.SrtSocket
}

// Listen creates a new SRT listener
func Listen(network, address string, config *Config) (Listener, error) {
	if network != "srt" {
		return Listener{}, fmt.Errorf("unsupported network: %s", network)
	}

	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return Listener{}, err
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return Listener{}, err
	}

	// Handle empty hostname - use localhost for SRT binding
	if host == "" {
		host = "0.0.0.0"
	}

	// Create socket options map
	options := make(map[string]string)
	if config.Passphrase != "" {
		options["passphrase"] = config.Passphrase
	}
	if config.PayloadSize > 0 {
		options["payloadsize"] = strconv.FormatUint(uint64(config.PayloadSize), 10)
	}

	socket := srtgo.NewSrtSocket(host, uint16(port), options)
	if socket == nil {
		return Listener{}, fmt.Errorf("failed to create SRT socket")
	}

	err = socket.Listen(1)
	if err != nil {
		socket.Close()
		return Listener{}, err
	}

	return Listener{socket: socket}, nil
}

// Accept2 accepts incoming connections and returns a ConnRequest
func (l Listener) Accept2() (ConnRequest, error) {
	conn, addr, err := l.socket.Accept()
	if err != nil {
		return ConnRequest{}, err
	}

	return ConnRequest{
		socket: conn,
		addr:   addr,
	}, nil
}

// Close closes the listener
func (l Listener) Close() error {
	if l.socket != nil {
		// Force close the socket immediately
		l.socket.Close()
	}
	return nil
}

// ConnRequest represents a connection request, compatible with gosrt.ConnRequest
type ConnRequest struct {
	socket *srtgo.SrtSocket
	addr   *net.UDPAddr
}

// RemoteAddr returns the remote address
func (cr ConnRequest) RemoteAddr() net.Addr {
	return cr.addr
}

// StreamId returns the stream ID
func (cr ConnRequest) StreamId() string {
	// Try to get stream ID from socket options
	if streamid, err := cr.socket.GetSockOptString(srtgo.SRTO_STREAMID); err == nil {
		return streamid
	}
	return ""
}

// IsEncrypted returns whether the connection is encrypted
func (cr ConnRequest) IsEncrypted() bool {
	// Check if passphrase is set
	if passphrase, err := cr.socket.GetSockOptString(srtgo.SRTO_PASSPHRASE); err == nil && passphrase != "" {
		return true
	}
	return false
}

// SetPassphrase sets the passphrase for the connection
func (cr ConnRequest) SetPassphrase(passphrase string) error {
	return cr.socket.SetSockOptString(srtgo.SRTO_PASSPHRASE, passphrase)
}

// Accept accepts the connection request
func (cr ConnRequest) Accept() (*Conn, error) {
	// Get local address from socket
	localAddr := &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"), // Will be updated with actual local IP
		Port: 0,                      // Will be updated with actual local port
	}

	return &Conn{
		socket:     cr.socket,
		localAddr:  localAddr,
		remoteAddr: cr.addr,
	}, nil
}

// Reject rejects the connection request
func (cr ConnRequest) Reject(reason int) {
	cr.socket.SetRejectReason(reason)
	cr.socket.Close()
}

// Conn represents an SRT connection, compatible with gosrt.Conn
type Conn struct {
	socket     *srtgo.SrtSocket
	localAddr  net.Addr
	remoteAddr net.Addr
}

// Read reads data from the connection
func (c *Conn) Read(b []byte) (int, error) {
	return c.socket.Read(b)
}

// Write writes data to the connection
func (c *Conn) Write(b []byte) (int, error) {
	return c.socket.Write(b)
}

// Close closes the connection
func (c *Conn) Close() error {
	if c.socket != nil {
		c.socket.Close()
	}
	return nil
}

// SetReadDeadline sets the read deadline
func (c *Conn) SetReadDeadline(t time.Time) error {
	c.socket.SetReadDeadline(t)
	return nil
}

// SetWriteDeadline sets the write deadline
func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.socket.SetWriteDeadline(t)
	return nil
}

// LocalAddr returns the local network address
func (c *Conn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr returns the remote network address
func (c *Conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// Stats retrieves connection statistics
func (c *Conn) Stats(stats *Statistics) error {
	srtStats, err := c.socket.Stats()
	if err != nil {
		return err
	}

	// Map srtgo.SrtStats to our Statistics structure
	stats.mapFromSrtgoStats(srtStats)
	return nil
}



// Dial creates a client connection to an SRT server
func Dial(network, address string, config *Config) (*Conn, error) {
	if network != "srt" {
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, err
	}

	// Handle empty hostname - use localhost for SRT connection
	if host == "" {
		host = "127.0.0.1"
	}

	// Create socket options map
	options := make(map[string]string)
	if config.Passphrase != "" {
		options["passphrase"] = config.Passphrase
	}
	if config.StreamID != "" {
		options["streamid"] = config.StreamID
	}
	if config.PayloadSize > 0 {
		options["payloadsize"] = strconv.FormatUint(uint64(config.PayloadSize), 10)
	}

	socket := srtgo.NewSrtSocket(host, uint16(port), options)
	if socket == nil {
		return nil, fmt.Errorf("failed to create SRT socket")
	}

	err = socket.Connect()
	if err != nil {
		socket.Close()
		return nil, err
	}

	// Create address structures
	remoteAddr := &net.UDPAddr{
		IP:   net.ParseIP(host),
		Port: int(port),
	}
	localAddr := &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"), // Will be updated with actual local IP
		Port: 0,                      // Will be updated with actual local port
	}

	return &Conn{
		socket:     socket,
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}, nil
}

// Statistics represents SRT connection statistics, compatible with gosrt.Statistics
type Statistics struct {
	// Accumulated statistics (compatible with gosrt)
	Accumulated struct {
		PktSent                uint64
		PktRecv                uint64
		PktSentUnique          uint64
		PktRecvUnique          uint64
		PktSendLoss            uint64
		PktRecvLoss            uint64
		PktRetrans             uint64
		PktRecvRetrans         uint64
		PktSentACK             uint64
		PktRecvACK             uint64
		PktSentNAK             uint64
		PktRecvNAK             uint64
		PktSentKM              uint64
		PktRecvKM              uint64
		UsSndDuration          uint64
		PktRecvBelated         uint64
		PktSendDrop            uint64
		PktRecvDrop            uint64
		PktRecvUndecrypt       uint64
		ByteSent               uint64
		ByteRecv               uint64
		ByteSentUnique         uint64
		ByteRecvUnique         uint64
		ByteRecvLoss           uint64
		ByteRetrans            uint64
		ByteRecvRetrans        uint64
		ByteRecvBelated        uint64
		ByteSendDrop           uint64
		ByteRecvDrop           uint64
		ByteRecvUndecrypt      uint64
	}

	// Instantaneous statistics (compatible with gosrt)
	Instantaneous struct {
		UsPktSendPeriod         float64
		PktFlowWindow           uint64
		PktFlightSize           uint64
		MsRTT                   float64
		MbpsSentRate            float64
		MbpsRecvRate            float64
		MbpsLinkCapacity        float64
		ByteAvailSendBuf        uint64
		ByteAvailRecvBuf        uint64
		MbpsMaxBW               float64
		ByteMSS                 uint64
		PktSendBuf              uint64
		ByteSendBuf             uint64
		MsSendBuf               uint64
		MsSendTsbPdDelay        uint64
		PktRecvBuf              uint64
		ByteRecvBuf             uint64
		MsRecvBuf               uint64
		MsRecvTsbPdDelay        uint64
		PktReorderTolerance     uint64
		PktRecvAvgBelatedTime   uint64
		PktSendLossRate         float64
		PktRecvLossRate         float64
	}
}

// mapFromSrtgoStats maps srtgo.SrtStats to our Statistics structure
func (s *Statistics) mapFromSrtgoStats(srtStats *srtgo.SrtStats) {
	// Map accumulated statistics
	s.Accumulated.PktSent = uint64(srtStats.PktSentTotal)
	s.Accumulated.PktRecv = uint64(srtStats.PktRecvTotal)
	s.Accumulated.PktSentUnique = uint64(srtStats.PktSent)
	s.Accumulated.PktRecvUnique = uint64(srtStats.PktRecv)
	s.Accumulated.PktSendLoss = uint64(srtStats.PktSndLossTotal)
	s.Accumulated.PktRecvLoss = uint64(srtStats.PktRcvLossTotal)
	s.Accumulated.PktRetrans = uint64(srtStats.PktRetransTotal)
	s.Accumulated.PktRecvRetrans = uint64(srtStats.PktRcvRetrans)
	s.Accumulated.PktSentACK = uint64(srtStats.PktSentACKTotal)
	s.Accumulated.PktRecvACK = uint64(srtStats.PktRecvACKTotal)
	s.Accumulated.PktSentNAK = uint64(srtStats.PktSentNAKTotal)
	s.Accumulated.PktRecvNAK = uint64(srtStats.PktRecvNAKTotal)
	s.Accumulated.UsSndDuration = uint64(srtStats.UsSndDurationTotal)
	s.Accumulated.PktRecvBelated = uint64(srtStats.PktRcvBelated)
	s.Accumulated.PktSendDrop = uint64(srtStats.PktSndDropTotal)
	s.Accumulated.PktRecvDrop = uint64(srtStats.PktRcvDropTotal)
	s.Accumulated.PktRecvUndecrypt = uint64(srtStats.PktRcvUndecryptTotal)
	s.Accumulated.ByteSent = uint64(srtStats.ByteSentTotal)
	s.Accumulated.ByteRecv = uint64(srtStats.ByteRecvTotal)
	s.Accumulated.ByteSentUnique = uint64(srtStats.ByteSent)
	s.Accumulated.ByteRecvUnique = uint64(srtStats.ByteRecv)
	s.Accumulated.ByteRecvLoss = uint64(srtStats.ByteRcvLossTotal)
	s.Accumulated.ByteRetrans = uint64(srtStats.ByteRetransTotal)
	s.Accumulated.ByteRecvRetrans = uint64(srtStats.ByteRetrans)
	s.Accumulated.ByteRecvBelated = uint64(srtStats.ByteRcvDrop) // Approximation
	s.Accumulated.ByteSendDrop = uint64(srtStats.ByteSndDropTotal)
	s.Accumulated.ByteRecvDrop = uint64(srtStats.ByteRcvDropTotal)
	s.Accumulated.ByteRecvUndecrypt = uint64(srtStats.ByteRcvUndecryptTotal)

	// Map instantaneous statistics
	s.Instantaneous.UsPktSendPeriod = srtStats.UsPktSndPeriod
	s.Instantaneous.PktFlowWindow = uint64(srtStats.PktFlowWindow)
	s.Instantaneous.PktFlightSize = uint64(srtStats.PktFlightSize)
	s.Instantaneous.MsRTT = srtStats.MsRTT
	s.Instantaneous.MbpsSentRate = srtStats.MbpsSendRate
	s.Instantaneous.MbpsRecvRate = srtStats.MbpsRecvRate
	s.Instantaneous.MbpsLinkCapacity = srtStats.MbpsBandwidth
	s.Instantaneous.ByteAvailSendBuf = uint64(srtStats.ByteAvailSndBuf)
	s.Instantaneous.ByteAvailRecvBuf = uint64(srtStats.ByteAvailRcvBuf)
	s.Instantaneous.MbpsMaxBW = srtStats.MbpsMaxBW
	s.Instantaneous.ByteMSS = uint64(srtStats.ByteMSS)
	s.Instantaneous.PktSendBuf = uint64(srtStats.PktSndBuf)
	s.Instantaneous.ByteSendBuf = uint64(srtStats.ByteSndBuf)
	s.Instantaneous.MsSendBuf = uint64(srtStats.MsSndBuf)
	s.Instantaneous.MsSendTsbPdDelay = uint64(srtStats.MsSndTsbPdDelay)
	s.Instantaneous.PktRecvBuf = uint64(srtStats.PktRcvBuf)
	s.Instantaneous.ByteRecvBuf = uint64(srtStats.ByteRcvBuf)
	s.Instantaneous.MsRecvBuf = uint64(srtStats.MsRcvBuf)
	s.Instantaneous.MsRecvTsbPdDelay = uint64(srtStats.MsRcvTsbPdDelay)
	s.Instantaneous.PktReorderTolerance = uint64(srtStats.PktReorderTolerance)
	s.Instantaneous.PktRecvAvgBelatedTime = uint64(srtStats.PktRcvAvgBelatedTime)
	// Loss rates need to be calculated from packet counts
	if srtStats.PktSent > 0 {
		s.Instantaneous.PktSendLossRate = float64(srtStats.PktSndLoss) / float64(srtStats.PktSent) * 100.0
	}
	if srtStats.PktRecv > 0 {
		s.Instantaneous.PktRecvLossRate = float64(srtStats.PktRcvLoss) / float64(srtStats.PktRecv) * 100.0
	}
}
