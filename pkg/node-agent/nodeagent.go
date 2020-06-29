package nodeagent

import (
	"context"

	"github.com/spf13/cobra"
)

var config = Config{}

var nodeAgentCmd = &cobra.Command{
	Use:   "node-agent",
	Short: "Starts the node agent",
	Long:  "The node agent runs on every Kubernetes cluster node to provide privileged system-level operations to the Istio agents running on the node",
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := NewServer(config)
		if err != nil {
			return err
		}
		server.Run(context.Background())
		return nil
	},
}

func init() {
	nodeAgentCmd.PersistentFlags().Uint16Var(&config.Port, "port", 1979, "port the server should listen on")
	nodeAgentCmd.PersistentFlags().StringVar(&config.CRISocketPath, "cri-socket-path", "/var/run/crio/crio.sock", "path to the CRI socket")
}

func GetCommand() *cobra.Command {
	return nodeAgentCmd
}
