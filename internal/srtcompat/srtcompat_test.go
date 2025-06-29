package srtcompat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfig_UnmarshalURL(t *testing.T) {
	config := DefaultConfig()
	
	// Test URL with streamid
	url := "srt://127.0.0.1:8890?streamid=publish:teststream:myuser:mypass:param=value"
	address, err := config.UnmarshalURL(url)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:8890", address)
	require.Equal(t, "publish:teststream:myuser:mypass:param=value", config.StreamID)
}

func TestConfig_UnmarshalURL_WithPassphrase(t *testing.T) {
	config := DefaultConfig()
	
	// Test URL with streamid and passphrase
	url := "srt://127.0.0.1:8890?streamid=teststream&passphrase=secret"
	address, err := config.UnmarshalURL(url)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:8890", address)
	require.Equal(t, "teststream", config.StreamID)
	require.Equal(t, "secret", config.Passphrase)
}

func TestConfig_UnmarshalURL_DefaultPort(t *testing.T) {
	config := DefaultConfig()
	
	// Test URL without port (should use default)
	url := "srt://127.0.0.1?streamid=teststream"
	address, err := config.UnmarshalURL(url)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:1935", address)
	require.Equal(t, "teststream", config.StreamID)
}

func TestConfig_Validate(t *testing.T) {
	config := DefaultConfig()
	err := config.Validate()
	require.NoError(t, err)
	
	// Test invalid timeout
	config.ConnectionTimeout = 0
	err = config.Validate()
	require.Error(t, err)
	
	// Test invalid payload size
	config.ConnectionTimeout = 3 * time.Second
	config.PayloadSize = 0
	err = config.Validate()
	require.Error(t, err)
}

func TestSRTInitCleanup(t *testing.T) {
	// Test that SRT init and cleanup don't crash
	InitSRT()
	CleanupSRT()
}

func TestLogLevel(t *testing.T) {
	// Test that setting log level doesn't crash
	SetLogLevel(LogLevelInfo)
	SetLogLevel(LogLevelDebug)
	SetLogLevel(LogLevelErr)
}
