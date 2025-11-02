// This file handles formatting of messages that go to the game

package main

import (
	"github.com/bwmarrin/discordgo"
	"regexp"
	"strings"
)

var (
	mentionPattern *regexp.Regexp
	rolePattern    *regexp.Regexp
	channelPattern *regexp.Regexp
)

//type Command struct {
//	Type    string `json:"type"`
//	User    string `json:"user"`
//	Message string `json:"msg"`
//}

func init() {
	mentionPattern, _ = regexp.Compile(`\\?<@!?\d+>`)
	rolePattern, _ = regexp.Compile(`\\?<@&\d+>`)
	channelPattern, _ = regexp.Compile(`\\?<#\d+>`)
}

func mentionTranslator(mentions []*discordgo.User, guild *discordgo.Guild) func(string) string {
	return func(match string) string {
		id := strings.Trim(match, "\\<@!>")
		for _, mention := range mentions {
			if mention.ID == id {
				return "@" + getUserNickname(mention, guild)
			}
		}
		return match
	}
}

func roleTranslator(guild *discordgo.Guild) func(string) string {
	return func(match string) string {
		roleID := strings.Trim(match, "\\<@&>")
		role, err := session.State.Role(guild.ID, roleID)
		if err == nil {
			return "@" + role.Name
		}
		return match
	}
}

func channelTranslator() func(string) string {
	return func(match string) string {
		id := strings.Trim(match, "\\<#>")
		if channel, err := session.State.Channel(id); err == nil {
			return "#" + channel.Name
		} else {
			return "#deleted-channel"
		}
	}
}

func getUnicodeToTextTranslator() *strings.Replacer {
	return strings.NewReplacer(
		"😃", ":)",
		"😄", ":D",
		"😦", ":(",
		"😐", ":|",
		"😛", ":P",
		"😉", ";)",
		"😭", ";(",
		"😠", ">:(",
		"😢", ":,(",
		"❤", "<3",
		"💔", "</3",
	)
}

// sanitizes special characters that could cause issues in the game
// This applies to both messages and usernames
func sanitizeForGame(text string) string {
	// Replace control characters and newlines that could break parsing or display
	replacer := strings.NewReplacer(
		"\n", " ",      // newlines to spaces
		"\r", " ",      // carriage returns to spaces
		"\t", " ",      // tabs to spaces
		"\x00", "",     // null bytes
		"\x1e", "",     // record separator (field delimiter used in log parsing)
	)
	return replacer.Replace(text)
}

// formats a discord message so it looks good in-game
func formatDiscordMessage(m *discordgo.MessageCreate) string {
	guild, err := getGuildForChannel(session, m.ChannelID)
	if err != nil {
		panic(err.Error())
	}
	message := mentionPattern.ReplaceAllStringFunc(m.Content, mentionTranslator(m.Mentions, guild))
	message = rolePattern.ReplaceAllStringFunc(message, roleTranslator(guild))
	message = channelPattern.ReplaceAllStringFunc(message, channelTranslator())
	message = getUnicodeToTextTranslator().Replace(message)
	message = sanitizeForGame(message)
	return message
}
