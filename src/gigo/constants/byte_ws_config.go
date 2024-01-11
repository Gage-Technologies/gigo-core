package constants

import "github.com/gage-technologies/gigo-lib/workspace_config"

var BytesWorkspaceConfig = workspace_config.GigoWorkspaceConfig{
	Version: 0.1,
	Resources: struct {
		CPU  int `yaml:"cpu"`
		Mem  int `yaml:"mem"`
		Disk int `yaml:"disk"`
		GPU  struct {
			Count int    `yaml:"count"`
			Class string `yaml:"class"`
		} `yaml:"gpu"`
	}{
		CPU:  1,  // number of CPUs
		Mem:  1,  // in GB
		Disk: 10, // in GB
		GPU: struct {
			Count int    `yaml:"count"`
			Class string `yaml:"class"`
		}{
			Count: 1,
			Class: "p4",
		},
	},
	BaseContainer:    "gigodev/gimg:bytes-base-ubuntu",
	WorkingDirectory: "/home/gigo/codebase/",
	Environment:      map[string]string{},
	Containers:       map[string]interface{}{},
	VSCode:           workspace_config.GigoVSCodeConfig{},
	PortForward:      []workspace_config.GigoPortForwardConfig{},
	Exec:             []workspace_config.GigoExecConfig{},
}
