package api

import (
	"net/http"

	"github.com/0xAtelerix/sdk/gosdk/rpc"
	"github.com/rs/zerolog"
)

// LoggingMiddleware logs every JSON-RPC request and response.
type LoggingMiddleware struct {
	log zerolog.Logger
}

func NewLoggingMiddleware(log zerolog.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{log: log}
}

func (m *LoggingMiddleware) ProcessRequest(_ http.ResponseWriter, r *http.Request) error {
	m.log.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote", r.RemoteAddr).
		Msg("RPC request")
	if r.Body == nil {
		return rpc.ErrNilRequestBody
	}
	return nil
}

func (m *LoggingMiddleware) ProcessResponse(_ http.ResponseWriter, _ *http.Request, resp rpc.JSONRPCResponse) error {
	if resp.Error != nil {
		m.log.Error().
			Interface("id", resp.ID).
			Str("error", resp.Error.Error()).
			Msg("RPC error response")
	} else {
		m.log.Debug().
			Interface("id", resp.ID).
			Msg("RPC success response")
	}
	return nil
}
