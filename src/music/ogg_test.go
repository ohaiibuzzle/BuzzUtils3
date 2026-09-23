package music

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// oggPage builds an Ogg page from lacing values and payload (CRC is not checked).
func oggPage(lacing []byte, payload []byte) []byte {
	header := make([]byte, 27)
	copy(header, "OggS")
	header[26] = byte(len(lacing))
	return append(append(header, lacing...), payload...)
}

func TestOggOpusReader(t *testing.T) {
	big := bytes.Repeat([]byte{'b'}, 300) // spans two pages: 255 + 45

	var stream []byte
	stream = append(stream, oggPage([]byte{19}, []byte("OpusHead___________"))...)
	stream = append(stream, oggPage([]byte{8}, []byte("OpusTags"))...)
	stream = append(stream, oggPage([]byte{3, 2, 255}, append([]byte("aaacc"), big[:255]...))...)
	stream = append(stream, oggPage([]byte{45, 0}, big[255:])...)

	r := newOggOpusReader(bytes.NewReader(stream))
	for _, want := range [][]byte{[]byte("aaa"), []byte("cc"), big, {}} {
		got, err := r.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("got %d bytes %q..., want %d bytes", len(got), got[:min(len(got), 5)], len(want))
		}
	}
	if _, err := r.Next(); err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}

func TestOggOpusReaderTruncated(t *testing.T) {
	r := newOggOpusReader(bytes.NewReader(oggPage([]byte{10}, []byte("short"))))
	if _, err := r.Next(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
	}
}
