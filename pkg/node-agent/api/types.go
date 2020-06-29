package api

type IPTablesRequest struct {
	PodNamespace string          `json:"podNamespace"`
	PodName      string          `json:"podName"`
	IPV4Options  IPTablesOptions `json:"ipv4Options"`
	IPV6Options  IPTablesOptions `json:"ipv6Options"`
}

type IPTablesOptions struct {
	Rules string `json:"rules"`
}

type IPTablesResponse struct {
	IPV4Result IPTablesResult `json:"ipv4"`
	IPV6Result IPTablesResult `json:"ipv6"`
}

type IPTablesResult struct {
	RestoreCommandOutput string `json:"restoreCommandOutput"`
	SaveCommandOutput    string `json:"saveCommandOutput"`
}
