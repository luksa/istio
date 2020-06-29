package nodeagent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"istio.io/pkg/log"

	"istio.io/istio/pkg/node-agent/api"
	"istio.io/istio/tools/istio-iptables/pkg/constants"
)

func (s *Server) handleIPTables(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	req := api.IPTablesRequest{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Infof("Error decoding iptables request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Infof("Handling iptables request for %s/%s", req.PodNamespace, req.PodName)

	targetPID, err := s.targetPIDProvider.GetTargetPID(ctx, req.PodName, req.PodNamespace)
	if err != nil {
		log.Errorf("Error getting target PID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := api.IPTablesResponse{}
	response.IPV4Result, err = s.runIPTables(ctx, constants.IPTABLESRESTORE, constants.IPTABLESSAVE, "ipv4", targetPID, req.IPV4Options)
	if err != nil {
		log.Errorf("Error running iptablesrestore: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response.IPV6Result, err = s.runIPTables(ctx, constants.IP6TABLESRESTORE, constants.IP6TABLESSAVE, "ipv6", targetPID, req.IPV6Options)
	if err != nil {
		log.Errorf("Error running ip6tablesrestore: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infof("Finished. Returning 200 OK")

	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	err = encoder.Encode(response)
	if err != nil {
		log.Errorf("Error writing response: %v", err)
		return
	}
}

func (s *Server) runIPTables(ctx context.Context, restoreCmd, saveCmd string, ipVersion string, targetPID string, req api.IPTablesOptions) (api.IPTablesResult, error) {
	res := api.IPTablesResult{}

	rulesFile, err := ioutil.TempFile("", fmt.Sprintf("%s-iptables-rules-%d.txt", ipVersion, time.Now().UnixNano()))
	if err != nil {
		return res, fmt.Errorf("unable to create iptables-restore file: %v", err)
	}
	log.Infof("Writing rules file %s with the following contents:\n%s", rulesFile.Name(), req.Rules)

	defer os.Remove(rulesFile.Name())
	if err := writeRulesFile(rulesFile, req.Rules); err != nil {
		return res, err
	}

	res.RestoreCommandOutput, err = executeInContainerNetNamespace(targetPID, restoreCmd, "--noflush", rulesFile.Name())
	if err != nil {
		return res, err
	}

	res.SaveCommandOutput, err = executeInContainerNetNamespace(targetPID, saveCmd)
	if err != nil {
		return res, err
	}

	return res, nil
}

func executeInContainerNetNamespace(targetPID string, commandAndArgs ...string) (string, error) {
	args := append([]string{"--target", targetPID, "-n"}, commandAndArgs...) // alternative: fmt.Sprintf("--net=%s", netNamespaceFile),

	log.Infof("Running nsenter %s", strings.Join(args, " "))
	output, err := exec.Command("nsenter", args...).CombinedOutput()

	if err != nil {
		log.Errorf("nsenter failed: %v", err)
		log.Infof("nsenter out: %s", output)
		return string(output), err
	}
	log.Infof("nsenter done: %s", output)
	return string(output), nil

}

func writeRulesFile(f *os.File, contents string) error {
	defer f.Close()
	writer := bufio.NewWriter(f)
	_, err := writer.WriteString(contents)
	if err != nil {
		return fmt.Errorf("unable to write iptables-restore file: %v", err)
	}
	err = writer.Flush()
	return err
}
