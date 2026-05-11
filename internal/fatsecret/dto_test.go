package fatsecret

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlexFloat_Unmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{`"150"`, 150},
		{`"27.17"`, 27.17},
		{`38`, 38},
		{`null`, 0},
		{`""`, 0},
		{`"0"`, 0},
	}
	for _, tc := range cases {
		var f flexFloat
		require.NoError(t, json.Unmarshal([]byte(tc.in), &f), "input=%s", tc.in)
		require.Equal(t, tc.want, float64(f), "input=%s", tc.in)
	}
}

func TestFlexFloat_BadInput(t *testing.T) {
	var f flexFloat
	require.Error(t, json.Unmarshal([]byte(`"not-a-number"`), &f))
}

func TestFlexFloat_Ptr(t *testing.T) {
	var zero flexFloat = 0
	require.Nil(t, zero.ptr())

	var v flexFloat = 1.5
	p := v.ptr()
	require.NotNil(t, p)
	require.Equal(t, 1.5, *p)
}

func TestFlexInt_Unmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{`"20583"`, 20583},
		{`20583`, 20583},
		{`null`, 0},
		{`""`, 0},
	}
	for _, tc := range cases {
		var i flexInt
		require.NoError(t, json.Unmarshal([]byte(tc.in), &i), "input=%s", tc.in)
		require.Equal(t, tc.want, int64(i))
	}
}

func TestNormalizeList(t *testing.T) {
	t.Run("null", func(t *testing.T) {
		require.Nil(t, normalizeList(json.RawMessage(`null`)))
	})
	t.Run("empty string", func(t *testing.T) {
		require.Nil(t, normalizeList(json.RawMessage(`""`)))
	})
	t.Run("empty bytes", func(t *testing.T) {
		require.Nil(t, normalizeList(nil))
	})
	t.Run("single object", func(t *testing.T) {
		out := normalizeList(json.RawMessage(`{"a":1}`))
		require.Len(t, out, 1)
		require.JSONEq(t, `{"a":1}`, string(out[0]))
	})
	t.Run("array", func(t *testing.T) {
		out := normalizeList(json.RawMessage(`[{"a":1},{"b":2}]`))
		require.Len(t, out, 2)
		require.JSONEq(t, `{"a":1}`, string(out[0]))
		require.JSONEq(t, `{"b":2}`, string(out[1]))
	})
	t.Run("whitespace around array", func(t *testing.T) {
		out := normalizeList(json.RawMessage(`  [{"a":1}]  `))
		require.Len(t, out, 1)
	})
}
