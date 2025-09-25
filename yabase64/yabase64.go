package yabase64

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
)

func Encode[T any](v T) (*bytes.Buffer, error) {
	var buf bytes.Buffer

	encoder := base64.NewEncoder(base64.StdEncoding, &buf)

	err := json.NewEncoder(encoder).Encode(v)
	if err != nil {
		return nil, err
	}

	if err := encoder.Close(); err != nil {
		return nil, err
	}

	return &buf, nil
}

func Decode[T any](value string) (*T, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}

	var result T

	err = json.NewDecoder(bytes.NewReader(decoded)).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
