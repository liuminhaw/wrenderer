package internal

import (
	"bytes"
	"compress/gzip"
	"fmt"
)

func Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(data)
	if err != nil {
		return nil, fmt.Errorf("compress: %w", err)
	}

    if err := writer.Close(); err != nil {
        return nil, fmt.Errorf("compress: %w", err)
    }

    return buf.Bytes(), nil
}

func Decompress(data []byte) ([]byte, error) {
    reader, err := gzip.NewReader(bytes.NewReader(data))
    if err != nil {
        return nil, fmt.Errorf("decompress: %w", err)
    }

    var buf bytes.Buffer
    if _, err := buf.ReadFrom(reader); err != nil {
        return nil, fmt.Errorf("decompress: %w", err)
    }

    return buf.Bytes(), nil
}
