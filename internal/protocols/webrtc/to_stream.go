package webrtc

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/rtptime"
	"github.com/bluenviron/mediamtx/internal/stream"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

var errNoSupportedCodecsTo = errors.New(
	"the stream doesn't contain any supported codec, which are currently " +
		"AV1, VP9, VP8, H265, H264, Opus, G722, G711, LPCM, KLV")

// ToStream maps a WebRTC connection to a MediaMTX stream.
func ToStream(
	pc *PeerConnection,
	stream **stream.Stream,
) ([]*description.Media, error) {
	var medias []*description.Media //nolint:prealloc
	timeDecoder := &rtptime.GlobalDecoder2{}
	timeDecoder.Initialize()

	for _, track := range pc.incomingTracks {
		var typ description.MediaType
		var forma format.Format

		switch strings.ToLower(track.track.Codec().MimeType) {
		case strings.ToLower(webrtc.MimeTypeAV1):
			typ = description.MediaTypeVideo
			forma = &format.AV1{
				PayloadTyp: uint8(track.track.PayloadType()),
			}

		case strings.ToLower(webrtc.MimeTypeVP9):
			typ = description.MediaTypeVideo
			forma = &format.VP9{
				PayloadTyp: uint8(track.track.PayloadType()),
			}

		case strings.ToLower(webrtc.MimeTypeVP8):
			typ = description.MediaTypeVideo
			forma = &format.VP8{
				PayloadTyp: uint8(track.track.PayloadType()),
			}

		case strings.ToLower(webrtc.MimeTypeH265):
			typ = description.MediaTypeVideo
			forma = &format.H265{
				PayloadTyp: uint8(track.track.PayloadType()),
			}

		case strings.ToLower(webrtc.MimeTypeH264):
			typ = description.MediaTypeVideo
			forma = &format.H264{
				PayloadTyp:        uint8(track.track.PayloadType()),
				PacketizationMode: 1,
			}

		case strings.ToLower(mimeTypeMultiopus):
			typ = description.MediaTypeAudio
			forma = &format.Opus{
				PayloadTyp:   uint8(track.track.PayloadType()),
				ChannelCount: int(track.track.Codec().Channels),
			}

		case strings.ToLower(webrtc.MimeTypeOpus):
			typ = description.MediaTypeAudio
			forma = &format.Opus{
				PayloadTyp: uint8(track.track.PayloadType()),
				ChannelCount: func() int {
					if strings.Contains(track.track.Codec().SDPFmtpLine, "stereo=1") {
						return 2
					}
					return 1
				}(),
			}

		case strings.ToLower(webrtc.MimeTypeG722):
			typ = description.MediaTypeAudio
			forma = &format.G722{}

		case strings.ToLower(webrtc.MimeTypePCMU):
			channels := int(track.track.Codec().Channels)
			if channels == 0 {
				channels = 1
			}

			typ = description.MediaTypeAudio
			forma = &format.G711{
				PayloadTyp: func() uint8 {
					if channels > 1 {
						return 118
					}
					return 0
				}(),
				MULaw:        true,
				SampleRate:   8000,
				ChannelCount: channels,
			}

		case strings.ToLower(webrtc.MimeTypePCMA):
			channels := int(track.track.Codec().Channels)
			if channels == 0 {
				channels = 1
			}

			typ = description.MediaTypeAudio
			forma = &format.G711{
				PayloadTyp: func() uint8 {
					if channels > 1 {
						return 119
					}
					return 8
				}(),
				MULaw:        false,
				SampleRate:   8000,
				ChannelCount: channels,
			}

		case strings.ToLower(mimeTypeL16):
			typ = description.MediaTypeAudio
			forma = &format.LPCM{
				PayloadTyp:   uint8(track.track.PayloadType()),
				BitDepth:     16,
				SampleRate:   int(track.track.Codec().ClockRate),
				ChannelCount: int(track.track.Codec().Channels),
			}

		default:
			return nil, fmt.Errorf("unsupported codec: %+v", track.track.Codec().RTPCodecCapability)
		}

		medi := &description.Media{
			Type:    typ,
			Formats: []format.Format{forma},
		}

		track.OnPacketRTP = func(pkt *rtp.Packet, ntp time.Time) {
			pts, ok := timeDecoder.Decode(track, pkt)
			if !ok {
				return
			}

			(*stream).WriteRTPPacket(medi, forma, pkt, ntp, pts)
		}

		medias = append(medias, medi)
	}

	// Check for KLV data channel only if we have KLV data in the stream
	klvMedia, err := setupKLVDataChannelForReading(pc, stream)
	if err != nil {
		return nil, err
	}
	if klvMedia != nil {
		medias = append(medias, klvMedia)
	}

	if len(medias) == 0 {
		return nil, errNoSupportedCodecsTo
	}

	return medias, nil
}

func setupKLVDataChannelForReading(pc *PeerConnection, stream **stream.Stream) (*description.Media, error) {
	// Only set up KLV data channel if stream is available
	if stream == nil || *stream == nil {
		return nil, nil
	}

	// Check if the stream actually has KLV data
	var hasKLV bool
	for _, media := range (*stream).Desc.Medias {
		for _, forma := range media.Formats {
			if _, ok := forma.(*format.KLV); ok {
				hasKLV = true
				break
			}
		}
		if hasKLV {
			break
		}
	}

	if !hasKLV {
		return nil, nil
	}

	// Create KLV format for data channel
	klvFormat := &format.KLV{
		PayloadTyp: 96, // Use dynamic payload type
	}

	klvMedia := &description.Media{
		Type:    description.MediaTypeApplication,
		Formats: []format.Format{klvFormat},
	}

	// Create KLV data channel handler for reading
	klvHandler := NewKLVDataChannelHandler(pc, pc.Log)
	err := klvHandler.SetupForReading(*stream, nil, klvFormat)
	if err != nil {
		return nil, err
	}

	return klvMedia, nil
}
