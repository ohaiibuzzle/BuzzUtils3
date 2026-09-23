package music

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// oggOpusReader splits an Ogg/Opus stream (as written by ffmpeg) into Opus
// packets. With -frame_duration 20, each packet is one 20ms frame, which is what
// Discord expects. The OpusHead and OpusTags header packets are skipped.
type oggOpusReader struct {
	r       *bufio.Reader
	header  [27]byte
	lacing  []byte // segment sizes of the current page
	partial []byte // a packet continuing onto the next page
}

func newOggOpusReader(r io.Reader) *oggOpusReader {
	return &oggOpusReader{r: bufio.NewReaderSize(r, 64<<10)}
}

// Next returns the next audio packet, or io.EOF at the end of the stream.
func (o *oggOpusReader) Next() ([]byte, error) {
	for {
		for len(o.lacing) > 0 {
			size := int(o.lacing[0])
			o.lacing = o.lacing[1:]
			start := len(o.partial)
			o.partial = append(o.partial, make([]byte, size)...)
			if _, err := io.ReadFull(o.r, o.partial[start:]); err != nil {
				return nil, unexpected(err)
			}
			// A segment shorter than 255 bytes ends the packet
			if size < 255 {
				packet := o.partial
				o.partial = nil
				if bytes.HasPrefix(packet, []byte("OpusHead")) || bytes.HasPrefix(packet, []byte("OpusTags")) {
					continue
				}
				return packet, nil
			}
		}
		if err := o.readPageHeader(); err != nil {
			return nil, err
		}
	}
}

func (o *oggOpusReader) readPageHeader() error {
	if _, err := io.ReadFull(o.r, o.header[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return io.EOF
		}
		return unexpected(err)
	}
	if !bytes.Equal(o.header[:4], []byte("OggS")) {
		return fmt.Errorf("not an Ogg page")
	}
	o.lacing = make([]byte, o.header[26])
	if _, err := io.ReadFull(o.r, o.lacing); err != nil {
		return unexpected(err)
	}
	return nil
}

func unexpected(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}
