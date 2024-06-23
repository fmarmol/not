package not

type Config struct {
	Commands      []string `toml:"commands"`
	Proxy         Proxy    `toml:"proxy"`
	ExcludedFiles []string `toml:"excluded_files"`
	Dirs          []Dir    `toml:"dirs"`
	Exts          []string `toml:"exts"`
}
