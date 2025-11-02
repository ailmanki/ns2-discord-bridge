// This file reads the log file of the server to parse messages that are to be shown in Discord

package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"
	"path/filepath"
	"strings"
)

const fieldSep = ""
const regexPrefix = "^\\[[0-9][0-9]:[0-9][0-9]:[0-9][0-9]\\]--DISCORD--\\|"

var (
	chatRegexp = regexp.MustCompile(regexPrefix + "chat" +
		fieldSep + "(.*?)" + // name
		fieldSep + "(.*?)" + // steam id
		fieldSep + "(.*?)" + // team number
		fieldSep + "(.*)\n") // message

	statusRegexp = regexp.MustCompile(regexPrefix + "status" +
		fieldSep + "(.*?)" + // status
		fieldSep + "(.*?)" + // map
		fieldSep + "(.*)\n") // player count

	changemapRegexp = regexp.MustCompile(regexPrefix + "changemap" +
		fieldSep + "(.*?)" + // map
		fieldSep + "(.*)\n") // player count

	initRegexp = regexp.MustCompile(regexPrefix + "init" +
		fieldSep + "(.*)\n") // map

	playerRegexp = regexp.MustCompile(regexPrefix + "player" +
		fieldSep + "(.*?)" + // action
		fieldSep + "(.*?)" + // name
		fieldSep + "(.*?)" + // steam id
		fieldSep + "(.*)\n") // player count

	adminprintRegexp = regexp.MustCompile(regexPrefix + "adminprint" +
		fieldSep + "(.*)\n") // message
)

func init() {
	// Log the regex patterns at startup for debugging
	log.Println("[LogParser] Field separator (hex):", strings.ToUpper(fmt.Sprintf("%x", fieldSep)))
	log.Println("[LogParser] Chat regex pattern:", chatRegexp.String())
}

func findLogFile(logpath string) string {
	if logpath == "" {
		log.Println("[LogParser] WARNING: log_file_path is empty in config!")
		return ""
	}
	
	log.Printf("[LogParser] Searching for log file based on path: %q", logpath)
	dir := filepath.Dir(logpath)
	log.Printf("[LogParser] Directory to search: %q", dir)
	
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Printf("[LogParser] WARNING: Directory does not exist: %q", dir)
		return ""
	}
	
	prefix := dir + string(os.PathSeparator) + "log-Server"
	log.Printf("[LogParser] Looking for files with prefix: %q", prefix)
	
	var file string
	var modTime time.Time
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("[LogParser] Error walking path %q: %v", path, err)
			return nil
		}
		switch {
		case path == dir:
		case info.Mode().IsDir():
			return filepath.SkipDir
		case strings.HasPrefix(path, prefix) && info.ModTime().After(modTime):
			log.Printf("[LogParser] Found candidate log file: %q (modified: %v)", path, info.ModTime())
			modTime = info.ModTime()
			file = path
		}
		return nil
	})

	if file == "" {
		log.Printf("[LogParser] WARNING: No log files found matching prefix %q", prefix)
	} else {
		log.Printf("[LogParser] Selected log file: %q", file)
	}
	return file
}

func startLogParser() {
	for serverName, server := range serverList {
		logfile := server.Config.LogFilePath
		log.Printf("[LogParser] Starting log parser for server '%s'", serverName)
		log.Printf("[LogParser] Configured log_file_path: %q", logfile)
		
		if logfile == "" {
			log.Printf("[LogParser] ERROR: log_file_path not configured for server '%s'", serverName)
			log.Printf("[LogParser] Please set log_file_path in your config.toml for this server")
			continue
		}
		
		currlog := findLogFile(logfile)
		if currlog == "" {
			log.Printf("[LogParser] ERROR: Could not find log file for server '%s'", serverName)
			continue
		}
		
		log.Printf("[LogParser] Monitoring log file: %s", currlog)
		file, err := os.Open(currlog)
		if err != nil {
			log.Printf("[LogParser] ERROR: Failed to open log file '%s': %v", currlog, err)
			continue
		}
		reader := bufio.NewReader(file)
		go func(serverName string, server *Server) {
			log.Printf("[LogParser] '%s': Skipping initial log content...", serverName)
			for { // Skip the initial stuff; yes, this isn't the most efficient way
				line, _ := reader.ReadString('\n')
				if len(line) == 0 {
					break
				}
			}
			log.Printf("[LogParser] '%s': Ready to process new log entries", serverName)

			var slept uint = 0
			//var filesize int64 = 0
			for {
				line, err := reader.ReadString('\n')
				if err != nil && len(line) == 0 {
					// End of file, will retry
					slept += 1
					time.Sleep(500 * time.Millisecond)
					continue
				}

				if len(line) != 0 {
					slept = 0
					//filesize = 0
					
					// Check if line contains DISCORD marker
					if strings.Contains(line, "--DISCORD--") {
						log.Printf("[LogParser] '%s': Found DISCORD line: %q", serverName, line)
						
						// Show the line with visible separators for debugging
						visibleLine := strings.ReplaceAll(line, "\x1e", "[SEP]")
						log.Printf("[LogParser] '%s': Line with visible separators: %q", serverName, visibleLine)
					}
					
					if matches := chatRegexp.FindStringSubmatch(line); matches != nil {
						log.Printf("[LogParser] '%s': Matched CHAT message - Name: %q, SteamID: %q, Team: %q, Message: %q", 
							serverName, matches[1], matches[2], matches[3], matches[4])
						log.Printf("[LogParser] '%s': Matched CHAT message - Name: %q, SteamID: %q, Team: %q, Message: %q", 
							serverName, matches[1], matches[2], matches[3], matches[4])
						steamid, _ := strconv.ParseInt(matches[2], 10, 32)
						teamNumber, _ := strconv.Atoi(matches[3])
						log.Printf("[LogParser] '%s': Forwarding chat message to Discord...", serverName)
						forwardChatMessageToDiscord(server, matches[1], SteamID3(steamid), TeamNumber(teamNumber), matches[4])
					} else if matches := statusRegexp.FindStringSubmatch(line); matches != nil {
						log.Printf("[LogParser] '%s': Matched STATUS message - State: %q, Map: %q, Players: %q", 
							serverName, matches[1], matches[2], matches[3])
						gamestate := matches[1]
						currmap := matches[2]
						players := matches[3]
						var message string
						var msgtype MessageType
						msgtype.GroupType = "status"
						switch gamestate {
						/* These are pretty much useless
						case "WarmUp":
							message          = "Warm-up started on "
							msgtype.SubType = "warmup"
						case "PreGame":
							message          = "Pregame started on "
							msgtype.SubType = "pregame"
						*/
						case "Started":
							message = "Round started on "
							msgtype.SubType = "roundstart"
						case "Team1Won":
							message = "Marines won on "
							msgtype.SubType = "marinewin"
						case "Team2Won":
							message = "Aliens won on "
							msgtype.SubType = "alienwin"
						case "Draw":
							message = "Draw on "
							msgtype.SubType = "draw"
						default:
							continue
						}
						log.Printf("[LogParser] '%s': Forwarding status message to Discord: %s", serverName, message+currmap)
						forwardStatusMessageToDiscord(server, msgtype, message, players, currmap)
					} else if matches := changemapRegexp.FindStringSubmatch(line); matches != nil {
						log.Printf("[LogParser] '%s': Matched CHANGEMAP - Map: %q, Players: %q", 
							serverName, matches[1], matches[2])
						nextmap := matches[1]
						players := matches[2]
						message := "Changing map to "
						log.Printf("[LogParser] '%s': Forwarding changemap to Discord", serverName)
						forwardStatusMessageToDiscord(server, MessageType{GroupType: "status", SubType: "changemap"}, message, players, nextmap)
					} else if matches := initRegexp.FindStringSubmatch(line); matches != nil {
						log.Printf("[LogParser] '%s': Matched INIT - Map: %q", serverName, matches[1])
						currmap := matches[1]
						message := "Loaded "
						log.Printf("[LogParser] '%s': Forwarding init to Discord", serverName)
						forwardStatusMessageToDiscord(server, MessageType{GroupType: "status", SubType: "init"}, message, "", currmap)
					} else if matches := playerRegexp.FindStringSubmatch(line); matches != nil {
						log.Printf("[LogParser] '%s': Matched PLAYER event - Action: %q, Name: %q, SteamID: %q, Players: %q", 
							serverName, matches[1], matches[2], matches[3], matches[4])
						action := matches[1]
						name := matches[2]
						steamid, _ := strconv.ParseInt(matches[3], 10, 32)
						players := matches[4]
						msgtype := MessageType{
							GroupType: "player",
							SubType:   action,
						}
						log.Printf("[LogParser] '%s': Forwarding player event to Discord", serverName)
						forwardPlayerEventToDiscord(server, msgtype, name, SteamID3(steamid), players)
					} else if matches := adminprintRegexp.FindStringSubmatch(line); matches != nil {
						log.Printf("[LogParser] '%s': Matched ADMINPRINT - Message: %q", serverName, matches[1])
						log.Printf("[LogParser] '%s': Forwarding adminprint to Discord", serverName)
						forwardStatusMessageToDiscord(server, MessageType{GroupType: "adminprint"}, matches[1], "", "")
					} else if strings.Contains(line, "--DISCORD--") {
						// Line contains DISCORD marker but didn't match any pattern
						log.Printf("[LogParser] '%s': WARNING - DISCORD line did not match any pattern!", serverName)
						log.Printf("[LogParser] '%s': Regex patterns expecting separator: %q", serverName, fieldSep)
					}
				} else if slept >= 5 { // Check if server has restarted
					slept = 0

					/*
						newlog := findLogFile(logfile)
						if newlog == currlog {
							newfile, err := os.Open(currlog)
							if err != nil {
								log.Println(err)
								continue
							}
							newstat, err := newfile.Stat()
							if err != nil {
								log.Println(err)
								continue
							}

							if filesize == 0 {
								filesize = newstat.Size()
							} else if newstat.Size() - filesize > 200 || filesize - newstat.Size() > 0 { // It is a new file
								log.Printf("Server restarted! (size changed, %v != %v)\n", filesize, newstat.Size())
								filesize = 0
								file.Close()
								file   = newfile
								reader = bufio.NewReader(file)
								forwardStatusMessageToDiscord(server, MessageType {GroupType: "status", SubType: "init"}, "Server restarted!", "", "")
							} else {
								time.Sleep(500 * time.Millisecond)
								newfile.Close()
							}
						} else {
							currlog = newlog
							newfile, err := os.Open(currlog)
							if err != nil {
								continue
							}
							filesize = 0
							log.Printf("Server restarted! (log file changed, %v != %v)", currlog, newlog)
							file.Close()
							file   = newfile
							reader = bufio.NewReader(file)
							forwardStatusMessageToDiscord(server, MessageType {GroupType: "status", SubType: "init"}, "Server restarted!", "", "")
						}
					*/
				} else {
					slept += 1
					time.Sleep(500 * time.Millisecond)
				}
			}
		}(serverName, server)
	}
}
