package models

type ConfigSetup struct {
	Compiler string
	Flags    string
	Name     string
}

type ConfigDependency struct {
	Target      string
	Sources     []string
	Includes    []string
	ObjectDpdcy bool
	BuildType   string
	Libs        string
}

type GomakeConfig struct {
	Constants    []string
	Setups       map[string]*ConfigSetup
	Dependencies map[string]*ConfigDependency
	Scripts      map[string]string
}
