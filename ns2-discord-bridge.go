package main

import (
	"flag"
	"log"
	"os/user"
)

const version = "v6.0.2"

var configFile string

func main() {
	// parse command line arguments
	flag.StringVar(&configFile, "c", "config.toml", "Specify Configuration File")
	flag.Parse()
	
	log.Println("Version", version)
	
	// Log current user information for debugging
	currentUser, err := user.Current()
	if err != nil {
		log.Printf("WARNING: Could not determine current user: %v", err)
	} else {
		log.Printf("Running as user: %s (UID: %s, GID: %s)", currentUser.Username, currentUser.Uid, currentUser.Gid)
		log.Printf("User home directory: %s", currentUser.HomeDir)
	}
	
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
