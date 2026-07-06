package valueparser_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/valueparser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseValue_IntegerOverflow(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "uint8 overflow returns error and zero value", run: testUint8OverflowReturnsError},
		{name: "int8 overflow returns error and zero value", run: testInt8OverflowReturnsError},
		{name: "uint16 overflow returns error and zero value", run: testUint16OverflowReturnsError},
		{name: "uint8 max value parses without error", run: testUint8MaxValueParses},
		{name: "int8 min value parses without error", run: testInt8MinValueParses},
		{name: "uint16 max value parses without error", run: testUint16MaxValueParses},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, testCase.run)
	}
}

func testUint8OverflowReturnsError(t *testing.T) {
	t.Parallel()

	const overflowingUint8 = "256"

	got, err := valueparser.ParseValue[uint8](overflowingUint8)

	require.Error(t, err)
	assert.Equal(t, uint8(0), got)
}

func testInt8OverflowReturnsError(t *testing.T) {
	t.Parallel()

	const overflowingInt8 = "200"

	got, err := valueparser.ParseValue[int8](overflowingInt8)

	require.Error(t, err)
	assert.Equal(t, int8(0), got)
}

func testUint16OverflowReturnsError(t *testing.T) {
	t.Parallel()

	const overflowingUint16 = "65536"

	got, err := valueparser.ParseValue[uint16](overflowingUint16)

	require.Error(t, err)
	assert.Equal(t, uint16(0), got)
}

func testUint8MaxValueParses(t *testing.T) {
	t.Parallel()

	const maxUint8 = "255"

	got, err := valueparser.ParseValue[uint8](maxUint8)

	require.NoError(t, err)
	assert.Equal(t, uint8(255), got)
}

func testInt8MinValueParses(t *testing.T) {
	t.Parallel()

	const minInt8 = "-128"

	got, err := valueparser.ParseValue[int8](minInt8)

	require.NoError(t, err)
	assert.Equal(t, int8(-128), got)
}

func testUint16MaxValueParses(t *testing.T) {
	t.Parallel()

	const maxUint16 = "65535"

	got, err := valueparser.ParseValue[uint16](maxUint16)

	require.NoError(t, err)
	assert.Equal(t, uint16(65535), got)
}
