package yagzip

import (
	"bytes"
	"compress/gzip"
	"io"
)

func Zip(object []byte) ([]byte, error) {
	var buf bytes.Buffer

	w := gzip.NewWriter(&buf)

	_, err := w.Write(object)
	if err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func Unzip(compressed []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var out bytes.Buffer
	_, err = io.Copy(&out, r)
	if err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}
