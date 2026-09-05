package transport

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"reflect"
	"testing"
)

func TestReadLimitedLineGrowthBoundaries(t *testing.T) {
	const limit = 1024
	for _, size := range []int{0, 15, 16, 17, 63, 64, 65, 511, 512, 513, limit - 1, limit, limit + 1} {
		for _, newline := range []bool{false, true} {
			payload := bytes.Repeat([]byte{'x'}, size)
			input := append([]byte(nil), payload...)
			if newline {
				input = append(input, '\n')
			}
			reader := bufio.NewReaderSize(bytes.NewReader(input), 16)
			line, oversized, err := readLimitedLine(reader, limit)
			if size > limit {
				if oversized == nil {
					t.Fatalf("size=%d newline=%v: missing oversize", size, newline)
				}
				continue
			}
			if oversized != nil || !bytes.Equal(line, payload) {
				t.Fatalf("size=%d newline=%v: incorrect frame", size, newline)
			}
			if err != nil && (size != 0 || newline || !errors.Is(err, io.EOF)) {
				t.Fatal(err)
			}
			if cap(line) > limit+1 {
				t.Fatalf("reserved %d beyond limit", cap(line))
			}
		}
	}
}

func TestReadLimitedLineOwnsReturnedStorage(t *testing.T) {
	first := bytes.Repeat([]byte{'a'}, 1024)
	second := bytes.Repeat([]byte{'b'}, 1024)
	input := append(append(append(append([]byte(nil), first...), '\n'), second...), '\n')
	reader := bufio.NewReaderSize(bytes.NewReader(input), 16)
	line1, _, err := readLimitedLine(reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	line2, _, err := readLimitedLine(reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(line1, first) || !bytes.Equal(line2, second) {
		t.Fatal("later read overwrote retained frame")
	}
	line2[0] = 'c'
	if !bytes.Equal(line1, first) {
		t.Fatal("frames share writable storage")
	}
}

// Keep the original append-based implementation as a differential oracle for
// framing, EOF, oversize metadata, and reader consumption across growth changes.
func referenceReadLimitedLine(r *bufio.Reader, limit int) ([]byte, *oversizedFrameInfo, error) {
	var line []byte
	for {
		fragment, err := r.ReadSlice('\n')
		line = append(line, fragment...)
		if lineExceedsLimit(line, limit) {
			return handleOversizedLine(r, err, line)
		}
		switch {
		case err == nil:
			return bytes.TrimSuffix(line, []byte{'\n'}), nil, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			if len(line) == 0 {
				return nil, nil, io.EOF
			}
			return line, nil, nil
		default:
			return nil, nil, err
		}
	}
}

func FuzzReadLimitedLineGrowth(f *testing.F) {
	for _, input := range []string{"", "a\nb\n", `{"id":1,"result":"abcdef"}`, "\n", string(bytes.Repeat([]byte{'x'}, 4096))} {
		f.Add([]byte(input), uint16(64), uint16(16))
	}
	f.Fuzz(func(t *testing.T, input []byte, limitSeed, bufferSeed uint16) {
		if len(input) > 65536 {
			t.Skip()
		}
		limit, bufferSize := int(limitSeed), 16+int(bufferSeed%4096)
		current := bufio.NewReaderSize(bytes.NewReader(input), bufferSize)
		reference := bufio.NewReaderSize(bytes.NewReader(input), bufferSize)
		for range 3 {
			got, gotOver, gotErr := readLimitedLine(current, limit)
			want, wantOver, wantErr := referenceReadLimitedLine(reference, limit)
			if !bytes.Equal(got, want) || !reflect.DeepEqual(gotOver, wantOver) || !errors.Is(gotErr, wantErr) {
				t.Fatalf("framing differs: errors=%v/%v oversize=%v/%v", gotErr, wantErr, gotOver, wantOver)
			}
		}
	})
}
