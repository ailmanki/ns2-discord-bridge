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
	"io"
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
	
	// First, try to check if the exact configured file exists
	if _, err := os.Stat(logpath); err == nil {
		log.Printf("[LogParser] NOTE: Configured file exists directly: %q", logpath)
		log.Printf("[LogParser] Using configured file directly instead of searching")
		return logpath
	} else {
		log.Printf("[LogParser] Configured file check: %v", err)
	}
	
	// Check if directory exists and is accessible
	dirInfo, err := os.Stat(dir)
	if os.IsNotExist(err) {
		log.Printf("[LogParser] ERROR: Directory does not exist: %q", dir)
		log.Printf("[LogParser] os.Stat error: %v (type: %T)", err, err)
		
		// Try to open directory to get more details
		if f, openErr := os.Open(dir); openErr != nil {
			log.Printf("[LogParser] os.Open error: %v (type: %T)", openErr, openErr)
		} else {
			f.Close()
			log.Printf("[LogParser] STRANGE: os.Open succeeded but os.Stat failed!")
		}
		
		// Try parent directory
		parent := filepath.Dir(dir)
		if parentInfo, parentErr := os.Stat(parent); parentErr != nil {
			log.Printf("[LogParser] Parent directory %q also not accessible: %v", parent, parentErr)
		} else {
			log.Printf("[LogParser] Parent directory %q is accessible (permissions: %v)", parent, parentInfo.Mode())
		}
		
		log.Printf("[LogParser] Please check your log_file_path setting in config.toml")
		log.Printf("[LogParser] The configured path was: %q", logpath)
		log.Printf("[LogParser] Possible causes:")
		log.Printf("[LogParser]   - Systemd sandboxing (PrivateTmp, ProtectHome, ProtectSystem, etc.)")
		log.Printf("[LogParser]   - Mount namespace isolation")
		log.Printf("[LogParser]   - Path doesn't exist in this process's view of filesystem")
		return ""
	} else if os.IsPermission(err) {
		log.Printf("[LogParser] ERROR: Permission denied accessing directory: %q", dir)
		log.Printf("[LogParser] The current user does not have permission to access this directory")
		log.Printf("[LogParser] Please check file/directory permissions or run as a different user")
		return ""
	} else if err != nil {
		log.Printf("[LogParser] ERROR: Cannot access directory %q: %v", dir, err)
		log.Printf("[LogParser] Error type: %T", err)
		return ""
	}
	
	log.Printf("[LogParser] Directory accessible - Permissions: %v", dirInfo.Mode())
	
	prefix := dir + string(os.PathSeparator) + "log-Server"
	log.Printf("[LogParser] Looking for files with prefix: %q", prefix)
	
	var file string
	var modTime time.Time
	fileCount := 0
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("[LogParser] Error walking path %q: %v", path, err)
			return nil
		}
		switch {
		case path == dir:
			// Skip the directory itself
		case info.Mode().IsDir():
			return filepath.SkipDir
		case strings.HasPrefix(path, prefix):
			fileCount++
			log.Printf("[LogParser] Found candidate log file: %q (modified: %v)", path, info.ModTime())
			if info.ModTime().After(modTime) {
				modTime = info.ModTime()
				file = path
			}
		}
		return nil
	})

	if file == "" {
		log.Printf("[LogParser] ERROR: No log files found matching prefix %q", prefix)
		log.Printf("[LogParser] Searched in directory: %q", dir)
		log.Printf("[LogParser] Files checked: %d", fileCount)
		log.Printf("[LogParser] Make sure NS2 server log files exist and are readable")
	} else {
		log.Printf("[LogParser] Selected log file: %q (most recent of %d files)", file, fileCount)
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
			log.Printf("[LogParser] Possible reasons:")
			log.Printf("[LogParser]   1. The log_file_path directory doesn't exist")
			log.Printf("[LogParser]   2. No log files matching 'log-Server*' in that directory")
			log.Printf("[LogParser]   3. Incorrect permissions to access the directory/files")
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
			for {
				line, err := reader.ReadString('\n')

				// --- 1. HANDLE ERRORS AND EOF ---
				if err != nil {
					if err == io.EOF {
						// End of file. This is normal.
						// Wait a bit, then check for rotation.
						slept += 1
						time.Sleep(500 * time.Millisecond)

						// Check if we've been idle long enough
						if slept >= 5 {
							slept = 0 // Reset counter

							// Get the file info of the currently open file handle
							// (This points to the *renamed* file)
							oldstat, statErr := file.Stat()
							if statErr != nil {
								log.Printf("[LogParser] '%s': Error stat'ing current file handle: %v", serverName, statErr)
								continue // Try again
							}

							// Check the file info at the *original configured path*
							// (This points to the *new* file)
							pathstat, pathErr := os.Stat(currlog)
							if pathErr != nil {
								log.Printf("[LogParser] '%s': Error stat'ing path %s: %v", serverName, currlog, pathErr)
								continue // Try again
							}

							// If they are not the same file, it was rotated!
							if !os.SameFile(oldstat, pathstat) {
								log.Printf("[LogParser] '%s': Log file was rotated, switching to new file at %s", serverName, currlog)
								newfile, openErr := os.Open(currlog)
								if openErr != nil {
									log.Printf("[LogParser] '%s': Error opening new log file: %v", serverName, openErr)
									continue // Try again
								}

								// Close the old file (server.log.1)
								file.Close() 
								
								// Start using the new file (server.log)
								file = newfile
								reader = bufio.NewReader(file)

								// *** IMPORTANT ***
								// We do NOT skip content. The new file is empty,
								// and we want to read it from the beginning.
								
								log.Printf("[LogParser] '%s': Ready to process new log entries after rotation", serverName)
								forwardStatusMessageToDiscord(server, MessageType{GroupType: "status", SubType: "init"}, "Server restarted/log rotated!", "", "")
							}
						}
					} else {
						// A real error, not just EOF
						log.Printf("[LogParser] '%s': Error reading log file: %v", serverName, err)
						time.Sleep(1 * time.Second) // Wait before retrying
					}
					
					// We had an error (EOF or other), so skip line processing
					continue 
				}

				// --- 2. PROCESS A VALID LINE ---
				// If we get here, err was nil and we have a line.
				
				slept = 0 // Reset the idle counter because we got data

				if len(line) == 0 {
					continue // Skip empty lines that somehow had no error
				}
				
				// (All your parsing logic below remains unchanged)

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
					log.Printf("[LogParser] '%s': WARNING - DISCORD line did not match any pattern!", serverName)
					log.Printf("[LogParser] '%s': Regex patterns expecting separator: %q", serverName, fieldSep)
				}
			} // end for
		}(serverName, server)
	}
}
