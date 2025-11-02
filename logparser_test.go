package main

import (
	"regexp"
	"testing"
)

// Test that the regex patterns correctly parse log messages with pipe separators
func TestLogParserRegexPatterns(t *testing.T) {
	tests := []struct {
		name        string
		regex       *regexp.Regexp
		input       string
		shouldMatch bool
		expected    []string // expected captured groups (excluding full match)
	}{
		{
			name:        "Chat message with pipe separators",
			regex:       chatRegexp,
			input:       "[11:26:54]--DISCORD--|chat|Meteru|235367954|0|test\n",
			shouldMatch: true,
			expected:    []string{"Meteru", "235367954", "0", "test"},
		},
		{
			name:        "Chat message with multi-word message",
			regex:       chatRegexp,
			input:       "[11:32:30]--DISCORD--|chat|PlayerName|12345|1|Hello World!\n",
			shouldMatch: true,
			expected:    []string{"PlayerName", "12345", "1", "Hello World!"},
		},
		{
			name:        "Player join event",
			regex:       playerRegexp,
			input:       "[11:32:09]--DISCORD--|player|join|Meteru|235367954|1/54\n",
			shouldMatch: true,
			expected:    []string{"join", "Meteru", "235367954", "1/54"},
		},
		{
			name:        "Player leave event",
			regex:       playerRegexp,
			input:       "[11:27:53]--DISCORD--|player|leave|Meteru|235367954|0/54\n",
			shouldMatch: true,
			expected:    []string{"leave", "Meteru", "235367954", "0/54"},
		},
		{
			name:        "Status message - round start",
			regex:       statusRegexp,
			input:       "[12:00:00]--DISCORD--|status|Started|ns2_summit|10/24\n",
			shouldMatch: true,
			expected:    []string{"Started", "ns2_summit", "10/24"},
		},
		{
			name:        "Change map event",
			regex:       changemapRegexp,
			input:       "[12:30:00]--DISCORD--|changemap|ns2_veil|8/24\n",
			shouldMatch: true,
			expected:    []string{"ns2_veil", "8/24"},
		},
		{
			name:        "Init event",
			regex:       initRegexp,
			input:       "[10:00:00]--DISCORD--|init|ns2_biodome\n",
			shouldMatch: true,
			expected:    []string{"ns2_biodome"},
		},
		{
			name:        "Admin print message",
			regex:       adminprintRegexp,
			input:       "[15:00:00]--DISCORD--|adminprint|Server message here\n",
			shouldMatch: true,
			expected:    []string{"Server message here"},
		},
		{
			name:        "Invalid message format - no pipes",
			regex:       chatRegexp,
			input:       "[11:26:54]--DISCORD--|chatMeteru2353679540test\n",
			shouldMatch: false,
		},
		{
			name:        "Invalid message format - wrong prefix",
			regex:       chatRegexp,
			input:       "[11:26:54]WRONG-PREFIX|chat|Meteru|235367954|0|test\n",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := tt.regex.FindStringSubmatch(tt.input)
			matched := matches != nil

			if matched != tt.shouldMatch {
				t.Errorf("Expected match=%v, got match=%v for input: %s", tt.shouldMatch, matched, tt.input)
				return
			}

			if matched && tt.shouldMatch {
				// Verify captured groups
				capturedGroups := matches[1:] // Exclude the full match
				if len(capturedGroups) != len(tt.expected) {
					t.Errorf("Expected %d captured groups, got %d. Groups: %v", len(tt.expected), len(capturedGroups), capturedGroups)
					return
				}

				for i, expected := range tt.expected {
					if capturedGroups[i] != expected {
						t.Errorf("Group %d: expected '%s', got '%s'", i, expected, capturedGroups[i])
					}
				}
			}
		})
	}
}

// Test field separator constant
func TestFieldSeparator(t *testing.T) {
	if fieldSep != "\\|" {
		t.Errorf("Expected fieldSep to be '\\|', got '%s'", fieldSep)
	}
}
