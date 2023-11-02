package cmd

// ExecECS is struct for login info to ECS Container
type ECSInfo struct {
	Cluster        string
	Service        string
	Task           string
	TaskDefinition string
	Container      string
	Command        string
	Shell          string
}

// ExecECSs is struct for list of ExecECS
type ECSInfos []ECSInfo
