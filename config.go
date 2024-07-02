package not

type CommandConfig struct {
	Cmd    string `toml:"command"`
	Deamon bool   `toml:"deamon"`
}

type Config struct {
	Commands      []CommandConfig `toml:"commands"`
	Proxy         Proxy           `toml:"proxy"`
	ExcludedFiles []string        `toml:"excluded_files"`
	Dirs          []Dir           `toml:"dirs"`
	Exts          []string        `toml:"exts"`
}
