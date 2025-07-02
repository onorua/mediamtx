package webrtc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/stream"
	"github.com/bluenviron/mediamtx/internal/unit"
	"github.com/pion/webrtc/v4"
)

const (
	klvDataChannelLabel = "klv"
)

// KLVDataChannelMessage represents a KLV message sent over data channel.
// Following ImpleoTV format with 'klvs' field containing parsed KLV data.
type KLVDataChannelMessage struct {
	Type string                 `json:"type"`
	KLVs map[string]interface{} `json:"klvs"`
}

// KLVDataChannelHandler handles KLV data streaming over WebRTC data channels.
type KLVDataChannelHandler struct {
	pc     *PeerConnection
	dc     *webrtc.DataChannel
	stream *stream.Stream
	reader stream.Reader
	format *format.KLV
	log    logger.Writer
	mutex  sync.RWMutex
	closed bool
}

// NewKLVDataChannelHandler creates a new KLV data channel handler.
func NewKLVDataChannelHandler(pc *PeerConnection, log logger.Writer) *KLVDataChannelHandler {
	return &KLVDataChannelHandler{
		pc:  pc,
		log: log,
	}
}

// SetupForPublishing sets up the data channel for publishing KLV data.
func (h *KLVDataChannelHandler) SetupForPublishing() error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.closed {
		return fmt.Errorf("handler is closed")
	}

	// Store the setup parameters, but defer actual data channel creation
	// until the peer connection is fully initialized
	return nil
}

// CreateDataChannelAfterStart creates the data channel after peer connection is started.
func (h *KLVDataChannelHandler) CreateDataChannelAfterStart() error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.closed {
		return fmt.Errorf("handler is closed")
	}

	if h.dc != nil {
		return nil // Already created
	}

	// Create data channel for KLV
	ordered := true
	maxRetransmits := uint16(0)
	dc, err := h.pc.CreateDataChannel(klvDataChannelLabel, &webrtc.DataChannelInit{
		Ordered:        &ordered,
		MaxRetransmits: &maxRetransmits,
	})
	if err != nil {
		return fmt.Errorf("failed to create KLV data channel: %w", err)
	}

	h.dc = dc
	h.setupDataChannelEvents()

	return nil
}

// SetupForReading sets up the data channel for reading KLV data.
func (h *KLVDataChannelHandler) SetupForReading(stream *stream.Stream, reader stream.Reader, klvFormat *format.KLV) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.closed {
		return fmt.Errorf("handler is closed")
	}

	h.stream = stream
	h.reader = reader
	h.format = klvFormat

	// Set up data channel handler for incoming data channels
	h.pc.SetOnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == klvDataChannelLabel {
			h.dc = dc
			h.setupDataChannelEvents()
		}
	})

	return nil
}

// SetDataChannel sets the data channel for this handler.
func (h *KLVDataChannelHandler) SetDataChannel(dc *webrtc.DataChannel) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.closed {
		return
	}

	h.dc = dc
	h.setupDataChannelEvents()
}

// setupDataChannelEvents sets up event handlers for the data channel.
func (h *KLVDataChannelHandler) setupDataChannelEvents() {
	if h.dc == nil {
		return
	}

	h.dc.OnOpen(func() {
		// KLV data channel opened
	})

	h.dc.OnClose(func() {
		// KLV data channel closed
	})

	h.dc.OnError(func(err error) {
		h.log.Log(logger.Warn, "KLV data channel error: %v", err)
	})

	h.dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		h.handleIncomingMessage(msg.Data)
	})
}

// handleIncomingMessage processes incoming KLV data from the data channel.
func (h *KLVDataChannelHandler) handleIncomingMessage(data []byte) {
	h.mutex.RLock()
	stream := h.stream
	format := h.format
	h.mutex.RUnlock()

	if stream == nil || format == nil {
		return
	}

	var msg KLVDataChannelMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		h.log.Log(logger.Warn, "failed to unmarshal KLV data channel message: %v", err)
		return
	}

	if msg.Type != "klv" {
		return
	}

	// Create KLV unit from received data
	// Extract data from the ImpleoTV-compatible format
	var klvData []byte
	var pts int64
	var ntp time.Time

	if rawData, ok := msg.KLVs["raw_data"].(string); ok {
		// Convert hex string back to bytes
		if data, err := hex.DecodeString(rawData); err == nil {
			klvData = data
		}
	}

	if ptsVal, ok := msg.KLVs["pts"].(float64); ok {
		pts = int64(ptsVal)
	}

	if ntpVal, ok := msg.KLVs["ntp"].(float64); ok {
		ntp = time.Unix(0, int64(ntpVal))
	}

	klvUnit := &unit.KLV{
		Base: unit.Base{
			NTP: ntp,
			PTS: pts,
		},
		Packets: klvData,
	}

	// Find the media for KLV format
	var media *description.Media
	for _, m := range stream.Desc.Medias {
		for _, f := range m.Formats {
			if f == format {
				media = m
				break
			}
		}
		if media != nil {
			break
		}
	}

	if media != nil {
		stream.WriteUnit(media, format, klvUnit)
	}
}

// SendKLVData sends KLV data over the data channel.
func (h *KLVDataChannelHandler) SendKLVData(u *unit.KLV) error {
	h.mutex.RLock()
	dc := h.dc
	closed := h.closed
	h.mutex.RUnlock()

	if closed || dc == nil {
		// Silently ignore if data channel is not available
		// This is normal during startup or when KLV is not enabled
		return nil
	}

	if dc.ReadyState() != webrtc.DataChannelStateOpen {
		// Silently ignore if data channel is not open yet
		// This prevents session closure during connection establishment
		return nil
	}

	// Create ImpleoTV-compatible message format
	// For now, send raw KLV data as hex string in klvs field
	// TODO: Add proper KLV parsing to extract individual key-value pairs
	klvs := map[string]interface{}{
		"raw_data": fmt.Sprintf("%x", u.Packets),
		"pts":      u.PTS,
		"ntp":      u.NTP.UnixNano(),
		"size":     len(u.Packets),
	}

	msg := KLVDataChannelMessage{
		Type: "klv",
		KLVs: klvs,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal KLV data: %w", err)
	}

	return dc.Send(data)
}

// Close closes the KLV data channel handler.
func (h *KLVDataChannelHandler) Close() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.closed = true
	if h.dc != nil {
		h.dc.Close() //nolint:errcheck
		h.dc = nil
	}
}

// IsReady returns true if the data channel is ready for use.
func (h *KLVDataChannelHandler) IsReady() bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return h.dc != nil && h.dc.ReadyState() == webrtc.DataChannelStateOpen
}
