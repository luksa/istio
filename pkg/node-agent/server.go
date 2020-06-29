package nodeagent

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"syscall"

	"istio.io/pkg/log"
)

// Config for the server.
type Config struct {
	LocalHostAddr string
	Port          uint16
	CRISocketPath string
}

// Server
type Server struct {
	mutex             sync.RWMutex
	port              uint16
	targetPIDProvider TargetPIDProvider
}

type TargetPIDProvider interface {
	GetTargetPID(ctx context.Context, podName, podNamespace string) (string, error)
}

// NewServer creates a new status server.
func NewServer(config Config) (*Server, error) {
	criAdapter, err := NewCRIAdapter(context.TODO(), config.CRISocketPath)
	if err != nil {
		return nil, err
	}

	s := &Server{
		port:              config.Port,
		targetPIDProvider: criAdapter,
	}
	return s, nil
}

func (s *Server) Run(ctx context.Context) {
	log.Infof("Starting Node agent on port %d\n", s.port)

	mux := http.NewServeMux()

	// Add the handler for ready probes.
	mux.HandleFunc("/iptables", s.handleIPTables)

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		log.Errorf("Error listening on status port: %v", err.Error())
		return
	}
	defer l.Close()

	go func() {
		if err := http.Serve(l, mux); err != nil {
			log.Errora(err)
			select {
			case <-ctx.Done():
				// We are shutting down already, don't trigger SIGTERM
				return
			default:
				// If the server errors then pilot-agent can never pass readiness or liveness probes
				// Therefore, trigger graceful termination by sending SIGTERM to the binary pid
				notifyExit()
			}
		}
	}()

	// Wait for the agent to be shut down.
	<-ctx.Done()
	log.Info("Node agent has successfully terminated")
}

// notifyExit sends SIGTERM to itself
func notifyExit() {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		log.Errora(err)
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		log.Errorf("failed to send SIGTERM to self: %v", err)
	}
}
