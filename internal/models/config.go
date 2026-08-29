package models

type ConfigName struct {
	Compiler string
	Flags    string
	Name     string
}

type ConfigDependency struct {
	Target   string
	Sources  []string
	Includes []string
	OFix     bool
}

type GomakeConfig struct {
	Name       ConfigName
	Dependency ConfigDependency
}
