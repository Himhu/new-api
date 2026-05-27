package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestToolCallArgumentsAcceptsStringForm(t *testing.T) {
	raw := []byte(`{"item":{"arguments":"{\"a\":1}"}}`)
	var resp ResponsesStreamResponse
	require.NoError(t, common.Unmarshal(raw, &resp))
	require.NotNil(t, resp.Item)
	require.Equal(t, `{"a":1}`, string(resp.Item.Arguments))
}

func TestToolCallArgumentsAcceptsObjectForm(t *testing.T) {
	raw := []byte(`{"item":{"arguments":{"a":1}}}`)
	var resp ResponsesStreamResponse
	require.NoError(t, common.Unmarshal(raw, &resp))
	require.NotNil(t, resp.Item)
	require.Equal(t, `{"a":1}`, string(resp.Item.Arguments))
}

func TestToolCallArgumentsAcceptsNullAndAbsent(t *testing.T) {
	rawNull := []byte(`{"item":{"arguments":null}}`)
	var respNull ResponsesStreamResponse
	require.NoError(t, common.Unmarshal(rawNull, &respNull))
	require.Equal(t, "", string(respNull.Item.Arguments))

	rawAbsent := []byte(`{"item":{}}`)
	var respAbsent ResponsesStreamResponse
	require.NoError(t, common.Unmarshal(rawAbsent, &respAbsent))
	require.Equal(t, "", string(respAbsent.Item.Arguments))
}

func TestToolCallArgumentsMarshalsAsString(t *testing.T) {
	out := ResponsesOutput{Arguments: ToolCallArguments(`{"a":1}`)}
	encoded, err := common.Marshal(out)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"arguments":"{\"a\":1}"`)
}
