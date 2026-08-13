package gracefulshutdown

import (
	"context"
	"errors"
	"net/http"
)

type httpServer struct {
	server *http.Server
}

// HTTPServer adapts net/http.Server to Component. It also works with Gin and
// other frameworks that expose an http.Handler.
func HTTPServer(server *http.Server) Component {
	return &httpServer{server: server}
}

func (s *httpServer) Start(context.Context) error {
	if s.server == nil {
		return errors.New("gracefulshutdown: nil http.Server")
	}
	err := s.server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *httpServer) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return errors.New("gracefulshutdown: nil http.Server")
	}
	return s.server.Shutdown(ctx)
}
