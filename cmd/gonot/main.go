package main

import (
	"log"
	"os"
	"strings"

	"github.com/fmarmol/not"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/pflag"
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func main() {

	const configFile = ".not.toml"

	// todo change in command
	i := pflag.BoolP("init", "i", false, "create empty configuration file")
	pflag.Parse()

	if *i {
		var cfg not.Config
		cfg.ExcludedFiles = []string{configFile}
		cfg.Commands = []not.CommandConfig{
			{`echo "hello world"`, false},
			{`echo "hello folks"`, false},
		}
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal("could not get working dir")
		}
		cfg.Dirs = []not.Dir{
			{Name: wd},
		}
		fd, err := os.Create(configFile)
		if err != nil {
			log.Fatal("coudl not create empty configuration file:", err)
		}
		defer fd.Close()
		err = toml.NewEncoder(fd).Encode(cfg)
		if err != nil {
			log.Fatal("coudl not create empty configuration file:", err)
		}
		return
	}

	fd, err := os.Open(configFile)
	if err != nil {
		log.Fatal("could not start not:", err)
	}
	var cfg not.Config
	err = toml.NewDecoder(fd).Decode(&cfg)
	if err != nil {
		log.Fatal("could not start not:", err)
	}

	opts := []not.WatchOpt{}
	for _, cmd := range cfg.Commands {
		cmdLine := strings.TrimSpace(cmd.Cmd)
		cmdLineSplit := strings.Split(cmdLine, " ")

		command := []string{}
		for _, c := range cmdLineSplit {
			if len(c) == 0 {
				continue
			}
			command = append(command, c)
		}
		opts = append(opts, not.CmdOpt(not.Cmd{
			Args:   command,
			Deamon: cmd.Deamon,
		}))
	}

	for _, ex := range cfg.ExcludedFiles {
		opts = append(opts, not.ExcludeFile(ex))
	}

	for _, dir := range cfg.Dirs {
		opts = append(opts, not.DirOpt(dir))
	}
	for _, ext := range cfg.Exts {
		opts = append(opts, not.ExtOpt(ext))
	}

	if cfg.Proxy.Activated {
		opts = append(opts, not.ProxyOpt(cfg.Proxy.PortApp, cfg.Proxy.PortNot))
	}
	w := not.NewWatcher(opts...)
	w.Run()
}
