package main

import (
	"flag"
	"log"
)

const version = "v6.0.2"

var configFile string

func main() {
	// parse command line arguments
	flag.StringVar(&configFile, "c", "config.toml", "Specify Configuration File")
	flag.Parse()
	
	log.Println("Version", version)
	Config.loadConfig(configFile)

	for serverName, v := range Config.Servers {
		serverList[serverName] = &Server{
			Name:   serverName,
			Config: v,
			Muted:  v.Muted,
		}
		log.Println("Linked server '"+serverName+"' to channel", v.ChannelID)
	}

	startDiscordBot()
	startLogParser()

	select {}
}
