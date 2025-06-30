package webrtc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/stream"
	"github.com/bluenviron/mediamtx/internal/unit"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
)

type testLogger struct{}

func (testLogger) Log(level logger.Level, format string, args ...interface{}) {}

func TestKLVDataChannelHandler_Creation(t *testing.T) {
	pc := &PeerConnection{
		Log: testLogger{},
	}

	handler := NewKLVDataChannelHandler(pc, testLogger{})
	require.NotNil(t, handler)
	require.Equal(t, pc, handler.pc)
	require.False(t, handler.IsReady())
}

func TestKLVDataChannelHandler_SetupForPublishing(t *testing.T) {
	// Create a real WebRTC peer connection for testing
	api := webrtc.NewAPI()
	wr, err := api.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer wr.Close()

	pc := &PeerConnection{
		wr:           wr,
		Log:          testLogger{},
		dataChannels: make(map[string]*webrtc.DataChannel),
	}

	handler := NewKLVDataChannelHandler(pc, testLogger{})
	err = handler.SetupForPublishing()
	require.NoError(t, err)

	// Create the data channel after setup
	err = handler.CreateDataChannelAfterStart()
	require.NoError(t, err)

	// Check that data channel was created
	dc := pc.GetDataChannel(klvDataChannelLabel)
	require.NotNil(t, dc)
	require.Equal(t, klvDataChannelLabel, dc.Label())
}

func TestKLVDataChannelHandler_SendKLVData(t *testing.T) {
	// Create a real WebRTC peer connection for testing
	api := webrtc.NewAPI()
	wr, err := api.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer wr.Close()

	pc := &PeerConnection{
		wr:           wr,
		Log:          testLogger{},
		dataChannels: make(map[string]*webrtc.DataChannel),
	}

	handler := NewKLVDataChannelHandler(pc, testLogger{})
	err = handler.SetupForPublishing()
	require.NoError(t, err)

	// Create test KLV unit
	klvUnit := &unit.KLV{
		Base: unit.Base{
			NTP: time.Now(),
			PTS: 1000000,
		},
		Packets: []byte{0x06, 0x0E, 0x2B, 0x34}, // Sample KLV data
	}

	// Test sending KLV data (will silently succeed since channel is not open)
	err = handler.SendKLVData(klvUnit)
	require.NoError(t, err) // Should not fail, just silently ignore when data channel is not open
}

func TestKLVDataChannelHandler_SetupForReading(t *testing.T) {
	// Create a real WebRTC peer connection for testing
	api := webrtc.NewAPI()
	wr, err := api.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer wr.Close()

	pc := &PeerConnection{
		wr:           wr,
		Log:          testLogger{},
		dataChannels: make(map[string]*webrtc.DataChannel),
	}

	// Create test stream and format
	klvCodec := &mpegts.CodecKLV{}
	klvFormat := &format.KLV{
		PayloadTyp: 96,
		KLVCodec:   klvCodec,
	}

	media := &description.Media{
		Type:    description.MediaTypeApplication,
		Formats: []format.Format{klvFormat},
	}

	desc := &description.Session{
		Medias: []*description.Media{media},
	}

	testStream := &stream.Stream{
		WriteQueueSize:     1024,
		UDPMaxPayloadSize:  1472,
		Desc:               desc,
		GenerateRTPPackets: true,
		Parent:             testLogger{},
	}
	err = testStream.Initialize()
	require.NoError(t, err)
	defer testStream.Close()

	handler := NewKLVDataChannelHandler(pc, testLogger{})
	err = handler.SetupForReading(testStream, nil, klvFormat)
	require.NoError(t, err)

	require.Equal(t, testStream, handler.stream)
	require.Equal(t, klvFormat, handler.format)
}

func TestKLVDataChannelHandler_Close(t *testing.T) {
	// Create a real WebRTC peer connection for testing
	api := webrtc.NewAPI()
	wr, err := api.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer wr.Close()

	pc := &PeerConnection{
		wr:           wr,
		Log:          testLogger{},
		dataChannels: make(map[string]*webrtc.DataChannel),
	}

	handler := NewKLVDataChannelHandler(pc, testLogger{})
	err = handler.SetupForPublishing()
	require.NoError(t, err)

	// Close the handler
	handler.Close()

	// Verify it's closed
	require.True(t, handler.closed)
	require.Nil(t, handler.dc)
}

func TestKLVDataChannelMessage_Serialization(t *testing.T) {
	msg := KLVDataChannelMessage{
		Type: "klv",
		KLVs: map[string]interface{}{
			"raw_data": "060e2b3401010101e01030101000000",
			"size":     16,
			"pts":      1000000,
			"ntp":      1234567890123456789,
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Test JSON unmarshaling
	var decoded KLVDataChannelMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	require.Equal(t, msg.Type, decoded.Type)
	require.Equal(t, msg.KLVs["raw_data"], decoded.KLVs["raw_data"])
	require.Equal(t, float64(16), decoded.KLVs["size"]) // JSON unmarshals numbers as float64
}
