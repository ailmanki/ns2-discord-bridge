package main

import (
	"flag"
	"log"
	"os"
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
	
	// Log current working directory for debugging
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("WARNING: Could not get current working directory: %v", err)
	} else {
		log.Printf("Current working directory: %s", cwd)
		
		// List contents of current directory
		entries, err := os.ReadDir(cwd)
		if err != nil {
			log.Printf("WARNING: Could not read working directory: %v", err)
		} else {
			log.Printf("Working directory contains %d entries:", len(entries))
			for i, entry := range entries {
				if i < 20 { // Limit to first 20 entries to avoid spam
					info, _ := entry.Info()
					if info != nil {
						log.Printf("  - %s (size: %d, mode: %v)", entry.Name(), info.Size(), info.Mode())
					} else {
						log.Printf("  - %s", entry.Name())
					}
				}
			}
			if len(entries) > 20 {
				log.Printf("  ... and %d more entries", len(entries)-20)
			}
		}
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
