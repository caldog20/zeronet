package header

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	HeaderVersion        = 1
	HeaderLen            = 16
	HeaderPadding uint16 = 0xfeed
)

// Message Types
const (
	None      uint8 = 0
	Handshake uint8 = 1
	Data      uint8 = 2
	Reset     uint8 = 3 // Redundant?
	Rekey     uint8 = 4
	Close     uint8 = 5
	Discovery uint8 = 6
	Punch     uint8 = 0xff
)

type Header struct {
	Version     uint8
	Type        uint8
	SenderIndex uint32
	Counter     uint64
	Padding     uint16
}

func NewHeader() *Header {
	return &Header{}
}

func (h *Header) Reset() {
	h.Version = HeaderVersion
	h.Type = 0
	h.SenderIndex = 0
	h.Counter = 0
	h.Padding = HeaderPadding
}

func (h *Header) Encode(b []byte, t uint8, index uint32, counter uint64) ([]byte, error) {
	if h == nil {
		return nil, errors.New("header cannot be nil")
	}

	if cap(b) < HeaderLen {
		return nil, errors.New("slice capacity too small to encode header")
	}

	h.Version = HeaderVersion
	h.Type = t
	h.SenderIndex = index
	h.Counter = counter
	h.Padding = HeaderPadding

	return encodeToSlice(b, h), nil
}

func encodeToSlice(b []byte, h *Header) []byte {
	b = b[:HeaderLen]
	b[0] = h.Version
	b[1] = h.Type
	binary.BigEndian.PutUint32(b[2:6], h.SenderIndex)
	binary.BigEndian.PutUint64(b[6:14], h.Counter)
	binary.BigEndian.PutUint16(b[14:16], h.Padding)

	return b
}

func (h *Header) Parse(b []byte) error {
	if h == nil {
		return errors.New("header cannot be nil")
	}

	if len(b) < HeaderLen {
		return errors.New("header length too short")
	}

	h.Version = b[0]
	h.Type = b[1]
	h.SenderIndex = binary.BigEndian.Uint32(b[2:6])
	h.Counter = binary.BigEndian.Uint64(b[6:14])
	h.Padding = binary.BigEndian.Uint16(b[14:16])

	if h.Version != HeaderVersion {
		return errors.New("header version mismatch")
	}

	if h.Padding != HeaderPadding {
		return errors.New("padding invalid, malformed header")
	}

	return nil
}

func (h *Header) String() string {
	if h == nil {
		return "<nil>"
	}

	return fmt.Sprintf(
		"header: {version: %d, type: %d, index: %d, counter :%d, padding: #%x",
		h.Version,
		h.Type,
		h.SenderIndex,
		h.Counter,
		h.Padding,
	)
}
