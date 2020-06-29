package nodeagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	cri "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"istio.io/pkg/log"
)

const maxMsgSize = 1024 * 1024 * 16

type CRIAdapter struct {
	criClient cri.RuntimeServiceClient
}

func NewCRIAdapter(ctx context.Context, criSocketPath string) (*CRIAdapter, error) {
	cc, err := dial(ctx, criSocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CRI server using socket file %q: %v", criSocketPath, err)
	}
	return &CRIAdapter{
		criClient: cri.NewRuntimeServiceClient(cc),
	}, nil
}

func dial(ctx context.Context, socketPath string) (*grpc.ClientConn, error) {
	return grpc.DialContext(ctx, socketPath, grpc.WithInsecure(), grpc.WithContextDialer(contextDialer), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)))
}

func contextDialer(ctx context.Context, path string) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		log.Warn("Failed to connect to CRI server",
			zap.String("endpoint", path),
			zap.Error(err))
	}
	return conn, err
}

func (c *CRIAdapter) GetTargetPID(ctx context.Context, podName, podNamespace string) (string, error) {
	sandboxStatus, err := c.findPodSandboxStatus(ctx, podName, podNamespace)
	if err != nil {
		return "", err
	}

	infoJSON := sandboxStatus.Info["info"]
	log.Debugf("Pod sandbox status info: %v", infoJSON)

	type statusInfo struct {
		PID uint64 `json:"pid"`
	}

	var info statusInfo
	err = json.Unmarshal([]byte(infoJSON), &info)
	if err != nil {
		return "", err
	}

	if info.PID == 0 {
		return "", fmt.Errorf("PID is zero")
	}

	pid := strconv.FormatUint(info.PID, 10)

	log.Infof("Pod sandbox PID: %s", pid)
	return pid, nil
}

func (c *CRIAdapter) findPodSandboxStatus(ctx context.Context, podName string, podNamespace string) (*cri.PodSandboxStatusResponse, error) {
	sandbox, err := c.findPodSandbox(ctx, podName, podNamespace)
	if err != nil {
		return nil, err
	}

	log.Infof("Found pod sandbox id for pod %s/%s: %s", podNamespace, podName, sandbox.Id)
	return c.criClient.PodSandboxStatus(ctx, &cri.PodSandboxStatusRequest{
		PodSandboxId: sandbox.Id,
		Verbose:      true,
	})
}

func (c *CRIAdapter) findPodSandbox(ctx context.Context, podName string, podNamespace string) (*cri.PodSandbox, error) {
	response, err := c.criClient.ListPodSandbox(ctx, &cri.ListPodSandboxRequest{})
	if err != nil {
		return nil, err
	}

	for _, sandbox := range response.Items {
		if sandbox.Metadata.Name == podName && sandbox.Metadata.Namespace == podNamespace {
			return sandbox, nil
		}
	}
	return nil, fmt.Errorf("No pod sandbox found for %s/%s", podNamespace, podName)
}
