// This file handles formatting of messages that go to the Discord chat

package main

import (
	"github.com/bwmarrin/discordgo"
	"math"
	"strconv"
	"strings"
	"time"
)

type TeamNumber int
type MessageType struct {
	GroupType string
	SubType   string
}

type ServerInfo struct {
	ServerIp   string                        `json:"serverIp"`
	ServerPort int                           `json:"serverPort"`
	ServerName string                        `json:"serverName"`
	Version    int                           `json:"version"`
	Mods       []ServerInfoModInfo           `json:"mods"`
	State      string                        `json:"state"`
	Map        string                        `json:"map"`
	GameTime   float64                       `json:"gameTime"`
	NumPlayers int                           `json:"numPlayers"`
	MaxPlayers int                           `json:"maxPlayers"`
	NumRookies int                           `json:"numRookies"`
	Teams      map[string]ServerInfoTeamInfo `json:"teams"`
}

type ServerInfoTeamInfo struct {
	TeamNumber int      `json:"teamNumber"`
	NumPlayers int      `json:"numPlayers"`
	NumRookies int      `json:"numRookies"`
	Players    []string `json:"players"`
}

type ServerInfoModInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

var (
	DefaultMessageColor      int = 75*256*256 + 78*256 + 82
	lastMultilineChatMessage *discordgo.Message
)

/* gets the guild icon for the supplied server
 * used for status messages
 */
func (messagetype MessageType) getIcon(server *Server) string {
	configuredIcon := server.Config.ServerIconUrl
	if configuredIcon != "" || server.Config.ChannelID == "" {
		return configuredIcon
	}
	guild, err := getGuildForChannel(session, server.Config.ChannelID)
	if err == nil {
		return "https://cdn.discordapp.com/icons/" + guild.ID + "/" + guild.Icon + ".png"
	}
	return ""
}

/* decides which color a message should get (rich message style only)
 * based on the message type
 */
func (messagetype MessageType) getColor() int {
	msgConfig := Config.MessageStyles.Rich
	switch messagetype.GroupType {
	case "player":
		switch messagetype.SubType {
		case "join":
			return Config.getColor(msgConfig.PlayerJoinColor, DefaultMessageColor)
		case "leave":
			return Config.getColor(msgConfig.PlayerLeaveColor, DefaultMessageColor)
		default:
			return Config.getColor(msgConfig.StatusColor, DefaultMessageColor)
		}
	case "info":
		fallthrough
	case "status":
		fallthrough
	case "adminprint":
		return Config.getColor(msgConfig.StatusColor, DefaultMessageColor)
	default:
		return DefaultMessageColor
	}
}

/* decides which color a message should get (rich message style only)
 * based on the team type
 */
func (teamNumber TeamNumber) getColor() int {
	msgConfig := Config.MessageStyles.Rich
	switch teamNumber {
	default:
		fallthrough
	case 0:
		return Config.getColor(msgConfig.ChatMessageReadyRoomColor, DefaultMessageColor)
	case 1:
		return Config.getColor(msgConfig.ChatMessageMarineColor, DefaultMessageColor)
	case 2:
		return Config.getColor(msgConfig.ChatMessageAlienColor, DefaultMessageColor)
	case 3:
		return Config.getColor(msgConfig.ChatMessageSpectatorColor, DefaultMessageColor)
	}
}

func (teamNumber TeamNumber) getPrefix() string {
	msgConfig := Config.MessageStyles.Text
	switch teamNumber {
	case 0:
		return msgConfig.ChatMessageReadyRoomPrefix
	case 1:
		return msgConfig.ChatMessageMarinePrefix
	case 2:
		return msgConfig.ChatMessageAlienPrefix
	case 3:
		return msgConfig.ChatMessageSpectatorPrefix
	default:
		return ""
	}
}

func getTextToUnicodeTranslator() *strings.Replacer {
	return strings.NewReplacer(
		":)", "😃",
		":D", "😄",
		":(", "😦",
		":|", "😐",
		":P", "😛",
		";)", "😉",
		";(", "😭",
		">:(", "😠",
		":,(", "😢",
		"<3", "❤",
		"</3", "💔",
	)
}

// escapes Discord markdown characters to prevent unintended formatting
// This is used for usernames and other text that shouldn't be formatted
func escapeDiscordMarkdown(text string) string {
	// First, validate and clean the text to prevent crashes
	text = sanitizeForDiscord(text)
	
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"*", "\\*",
		"_", "\\_",
		"~", "\\~",
		"`", "\\`",
		"|", "\\|",
	)
	text = replacer.Replace(text)
	
	// Enforce Discord's author name limit (256 characters)
	return truncateUTF8(text, 256)
}

// sanitizeForDiscord removes problematic characters that could crash Discord API calls
func sanitizeForDiscord(text string) string {
	// Remove null bytes and other control characters that could break JSON encoding
	replacer := strings.NewReplacer(
		"\x00", "",  // null bytes
		"\x01", "",  // start of heading
		"\x02", "",  // start of text
		"\x03", "",  // end of text
		"\x04", "",  // end of transmission
		"\x05", "",  // enquiry
		"\x06", "",  // acknowledge
		"\x07", "",  // bell
		"\x08", "",  // backspace
		"\x0b", "",  // vertical tab
		"\x0c", "",  // form feed
		"\x0e", "",  // shift out
		"\x0f", "",  // shift in
		"\x10", "",  // data link escape
		"\x11", "",  // device control 1
		"\x12", "",  // device control 2
		"\x13", "",  // device control 3
		"\x14", "",  // device control 4
		"\x15", "",  // negative acknowledge
		"\x16", "",  // synchronous idle
		"\x17", "",  // end of transmission block
		"\x18", "",  // cancel
		"\x19", "",  // end of medium
		"\x1a", "",  // substitute
		"\x1b", "",  // escape
		"\x1c", "",  // file separator
		"\x1d", "",  // group separator
		"\x1e", "",  // record separator
		"\x1f", "",  // unit separator
	)
	text = replacer.Replace(text)
	
	// Ensure the text is valid UTF-8 by replacing invalid sequences
	text = strings.ToValidUTF8(text, "")
	
	return text
}

// truncateUTF8 safely truncates a UTF-8 string to maxBytes without breaking multi-byte characters
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	
	// Ensure we don't go beyond the string length
	if maxBytes > len(s) {
		maxBytes = len(s)
	}
	
	// Find the last valid rune boundary before maxBytes
	for i := maxBytes; i > 0; i-- {
		if (s[i] & 0xC0) != 0x80 {
			// Found the start of a rune
			return s[:i]
		}
	}
	return ""
}

// sanitizePlayerNames sanitizes a list of player names for Discord display
func sanitizePlayerNames(players []string) []string {
	sanitized := make([]string, len(players))
	for i, name := range players {
		sanitized[i] = escapeDiscordMarkdown(name)
	}
	return sanitized
}

func buildTextChatMessage(server *Server, username string, teamNumber TeamNumber, message string) string {
	messageFormat := Config.MessageStyles.Text.ChatMessageFormat
	teamSpecificString := teamNumber.getPrefix()
	serverSpecificString := server.Config.ServerChatMessagePrefix
	replacer := strings.NewReplacer("%p", username, "%m", message, "%t", teamSpecificString, "%s", serverSpecificString)
	formattedMessage := replacer.Replace(messageFormat)
	return formattedMessage
}

func buildTextPlayerEvent(server *Server, messagetype MessageType, username string, message string) string {
	messageConfig := Config.MessageStyles.Text
	messageFormat := "%s %p %m"
	switch messagetype.SubType {
	case "join":
		messageFormat = messageConfig.PlayerJoinFormat
	case "leave":
		messageFormat = messageConfig.PlayerLeaveFormat
	}
	serverSpecificString := server.Config.ServerChatMessagePrefix
	replacer := strings.NewReplacer("%p", username, "%m", message, "%s", serverSpecificString)
	formattedMessage := replacer.Replace(messageFormat)
	return formattedMessage
}

func getLastMessageID(channelID string) (string, bool) {
	messages, _ := session.ChannelMessages(channelID, 1, "", "", "")
	if len(messages) == 1 {
		return messages[0].ID, true
	}
	return "", false
}

func findKeywordNotifications(server *Server, message string) (found bool, response string) {
	guild, err := getGuildForChannel(session, server.Config.ChannelID)
	if err != nil {
		return false, ""
	}

	fields := strings.Fields(message)
	keywordMapping := server.Config.KeywordNotifications
	for i := 0; i < len(keywordMapping); i += 2 {
		keywords := keywordMapping[i]
		mentions := keywordMapping[i+1]
		for _, keyword := range keywords {
			for _, field := range fields {
				if field == string(keyword) {
					response += mentions.toMentionString(guild)
					found = true
				}
			}
		}
	}
	return
}

func triggerKeywords(server *Server, message string) {
	if keywordsFound, mentions := findKeywordNotifications(server, message); keywordsFound && mentions != "" {
		_, _ = session.ChannelMessageSend(server.Config.ChannelID, mentions)
	}
}

func forwardChatMessageToDiscord(server *Server, username string, steamID SteamID3, teamNumber TeamNumber, message string) {
	// Sanitize message content to prevent crashes from special characters
	message = sanitizeForDiscord(message)
	translatedMessage := getTextToUnicodeTranslator().Replace(message)
	// Enforce Discord's embed description limit (4096 characters)
	translatedMessage = truncateUTF8(translatedMessage, 4096)
	escapedUsername := escapeDiscordMarkdown(username)
	switch Config.Discord.MessageStyle {
	default:
		fallthrough
	case "multiline":
		lastMessageID, ok := getLastMessageID(server.Config.ChannelID)
		if ok && lastMultilineChatMessage != nil {
			lastEmbed := lastMultilineChatMessage.Embeds[0]
			lastAuthor := lastEmbed.Author
			if lastMessageID == lastMultilineChatMessage.ID &&
				lastEmbed.Color == teamNumber.getColor() &&
				lastAuthor.Name == escapedUsername &&
				lastAuthor.URL == steamID.getSteamProfileLink() {
				// append to last message
				lastEmbed.Description += "\n" + translatedMessage
				lastMultilineChatMessage, _ = session.ChannelMessageEditEmbed(server.Config.ChannelID, lastMessageID, lastEmbed)
				triggerKeywords(server, translatedMessage)
				return
			}
		}
		embed := &discordgo.MessageEmbed{
			Description: translatedMessage,
			Color:       teamNumber.getColor(),
			Author: &discordgo.MessageEmbedAuthor{
				URL:     steamID.getSteamProfileLink(),
				Name:    escapedUsername,
				IconURL: steamID.getAvatar(),
			},
		}
		lastMultilineChatMessage, _ = session.ChannelMessageSendEmbed(server.Config.ChannelID, embed)

	case "oneline":
		embed := &discordgo.MessageEmbed{
			Color: teamNumber.getColor(),
			Footer: &discordgo.MessageEmbedFooter{
				Text:    escapedUsername + ": " + translatedMessage,
				IconURL: steamID.getAvatar(),
			},
		}
		_, _ = session.ChannelMessageSendEmbed(server.Config.ChannelID, embed)

	case "text":
		_, _ = session.ChannelMessageSend(server.Config.ChannelID, buildTextChatMessage(server, escapedUsername, teamNumber, translatedMessage))
	}

	triggerKeywords(server, translatedMessage)
}

func forwardPlayerEventToDiscord(server *Server, messagetype MessageType, username string, steamID SteamID3, playerCount string) {
	timestamp := ""
	switch messagetype.SubType + strings.Split(playerCount, "/")[0] {
	case "join1":
		fallthrough
	case "leave0":
		timestamp = time.Now().UTC().Format("2006-01-02T15:04:05")
	}

	if playerCount != "" {
		playerCount = " (" + playerCount + ")"
	}

	escapedUsername := escapeDiscordMarkdown(username)
	eventText := ""
	switch messagetype.SubType {
	case "join":
		eventText = escapedUsername + " joined" + playerCount
	case "leave":
		eventText = escapedUsername + " left" + playerCount
	}
	
	// Discord footer text has a 2048 character limit
	eventText = truncateUTF8(eventText, 2048)

	switch Config.Discord.MessageStyle {
	default:
		fallthrough
	case "multiline":
		fallthrough
	case "oneline":
		embed := &discordgo.MessageEmbed{
			Timestamp: timestamp,
			Color:     messagetype.getColor(),
			Footer: &discordgo.MessageEmbedFooter{
				Text:    eventText,
				IconURL: steamID.getAvatar(),
			},
		}
		_, _ = session.ChannelMessageSendEmbed(server.Config.ChannelID, embed)

	case "text":
		_, _ = session.ChannelMessageSend(server.Config.ChannelID, buildTextPlayerEvent(server, messagetype, escapedUsername, playerCount))
	}
}

func forwardStatusMessageToDiscord(server *Server, messagetype MessageType, message string, playerCount string, mapname string) {
	// Sanitize map name and status message to prevent crashes
	mapname = sanitizeForDiscord(mapname)
	message = sanitizeForDiscord(message)
	
	message += mapname

	if playerCount != "" {
		message += " (" + playerCount + ")"
	}
	
	// Discord footer text has a 2048 character limit
	message = truncateUTF8(message, 2048)

	statusChannelID := server.Config.StatusChannelID

	switch Config.Discord.MessageStyle {
	default:
		fallthrough
	case "multiline":
		fallthrough
	case "oneline":
		timestamp := ""
		switch messagetype.SubType {
		case "roundstart":
			fallthrough
		case "marinewin":
			fallthrough
		case "alienwin":
			fallthrough
		case "draw":
			timestamp = time.Now().UTC().Format("2006-01-02T15:04:05")
		}
		embed := &discordgo.MessageEmbed{
			Timestamp: timestamp,
			Color:     messagetype.getColor(),
			Footer: &discordgo.MessageEmbedFooter{
				Text:    message,
				IconURL: messagetype.getIcon(server),
			},
		}
		_, _ = session.ChannelMessageSendEmbed(server.Config.ChannelID, embed)

		if statusChannelID != "" && statusChannelID != server.Config.ChannelID {
			_, _ = session.ChannelMessageSendEmbed(statusChannelID, embed)
		}

	case "text":
		_, _ = session.ChannelMessageSend(server.Config.ChannelID, server.Config.ServerStatusMessagePrefix+message)

		if statusChannelID != "" && statusChannelID != server.Config.ChannelID {
			_, _ = session.ChannelMessageSend(statusChannelID, server.Config.ServerStatusMessagePrefix+message)
		}
	}

	if messagetype.SubType == "changemap" {
		if len(serverList) == 1 {
			session.UpdateGameStatus(0, mapname)
			// session.UpdateStreamingStatus(0, "Natural Selection 2", "https://www.twitch.tv/naturalselection2")
		} else {
			session.UpdateGameStatus(0, "")
		}
	}
}

func forwardServerStatusToDiscord(server *Server, messagetype MessageType, info ServerInfo) {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05")
	gameTimeSec, _ := math.Modf(info.GameTime)
	
	// Sanitize map and state names to prevent crashes
	info.Map = sanitizeForDiscord(info.Map)
	info.State = sanitizeForDiscord(info.State)
	
	description := ""
	description += "**Map:** " + info.Map
	description += "\n**State:** " + info.State + " (" + strconv.Itoa(int(gameTimeSec/60)) + "m " + strconv.Itoa(int(gameTimeSec)%60) + "s)"
	description += "\n**Players:** " + strconv.Itoa(info.NumPlayers) + "/" + strconv.Itoa(info.MaxPlayers)

	// if messagetype.SubType == "status" {
	// description += "\n​\t​\t​\t​\t​\t`Marines ______` "+ strconv.Itoa(info.Teams["1"].NumPlayers) + " Players"
	// description += "\n​\t​\t​\t​\t​\t`Aliens________` "+ strconv.Itoa(info.Teams["2"].NumPlayers) + " Players"
	// description += "\n​\t​\t​\t​\t​\t`ReadyRoom ____` "+ strconv.Itoa(info.Teams["0"].NumPlayers) + " Players"
	// description += "\n​\t​\t​\t​\t​\t`Spectators____`"+ strconv.Itoa(info.Teams["3"].NumPlayers) + " Players"
	// }

	if messagetype.SubType == "info" {
		description += "\n**Rookies:** " + strconv.Itoa(info.NumRookies)
		description += "\n**Version:** " + strconv.Itoa(info.Version)
	}

	fields := make([]*discordgo.MessageEmbedField, 0)

	if messagetype.SubType == "info" && len(info.Teams) == 4 {
		// Sanitize player names in each team using helper function
		marineTeam := &discordgo.MessageEmbedField{
			Name:   "Marines (" + strconv.Itoa(info.Teams["1"].NumPlayers) + " Players)",
			Value:  "​" + strings.Join(sanitizePlayerNames(info.Teams["1"].Players), "\n"),
			Inline: true,
		}
		// Discord field value has a 1024 character limit
		marineTeam.Value = truncateUTF8(marineTeam.Value, 1024)
		fields = append(fields, marineTeam)

		alienTeam := &discordgo.MessageEmbedField{
			Name:   "Aliens (" + strconv.Itoa(info.Teams["2"].NumPlayers) + " Players)",
			Value:  "​" + strings.Join(sanitizePlayerNames(info.Teams["2"].Players), "\n"),
			Inline: true,
		}
		alienTeam.Value = truncateUTF8(alienTeam.Value, 1024)
		fields = append(fields, alienTeam)

		lineBreak := &discordgo.MessageEmbedField{
			Name:   "​",
			Value:  "​",
			Inline: false,
		}
		fields = append(fields, lineBreak)

		rrTeam := &discordgo.MessageEmbedField{
			Name:   "ReadyRoom (" + strconv.Itoa(info.Teams["0"].NumPlayers) + " Players)",
			Value:  "​" + strings.Join(sanitizePlayerNames(info.Teams["0"].Players), "\n"),
			Inline: true,
		}
		rrTeam.Value = truncateUTF8(rrTeam.Value, 1024)
		fields = append(fields, rrTeam)

		specTeam := &discordgo.MessageEmbedField{
			Name:   "Spectators (" + strconv.Itoa(info.Teams["3"].NumPlayers) + " Players)",
			Value:  "​" + strings.Join(sanitizePlayerNames(info.Teams["3"].Players), "\n"),
			Inline: true,
		}
		specTeam.Value = truncateUTF8(specTeam.Value, 1024)
		fields = append(fields, specTeam)

		mods := make([]string, 0)
		for _, v := range info.Mods {
			mods = append(mods, sanitizeForDiscord(v.Name))
		}
		modsField := &discordgo.MessageEmbedField{
			Name:   "Mods",
			Value:  "​" + strings.Join(mods[:], "\n"),
			Inline: false,
		}
		modsField.Value = truncateUTF8(modsField.Value, 1024)
		fields = append(fields, modsField)
	}

	// Sanitize server name and info
	serverName := sanitizeForDiscord(server.Name)
	serverIpPort := sanitizeForDiscord(info.ServerIp + ":" + strconv.Itoa(info.ServerPort))
	
	// Enforce Discord limits with safe UTF-8 truncation
	serverName = truncateUTF8(serverName, 256)
	description = truncateUTF8(description, 4096)
	serverIpPort = truncateUTF8(serverIpPort, 2048)

	embed := &discordgo.MessageEmbed{
		Color: messagetype.getColor(),
		Author: &discordgo.MessageEmbedAuthor{
			Name:    serverName,
			IconURL: messagetype.getIcon(server),
		},
		Description: description,
		Fields:      fields,
		Timestamp:   timestamp,
		Footer: &discordgo.MessageEmbedFooter{
			Text: serverIpPort,
		},
	}
	_, _ = session.ChannelMessageSendEmbed(server.Config.ChannelID, embed)
}
