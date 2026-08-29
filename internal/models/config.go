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
}

type GomakeConfig struct {
	Setup      ConfigSetup
	Dependency ConfigDependency
	Scripts    map[string]string
}
