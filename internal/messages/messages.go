package messages

import "strings"

const DefaultMaxLength = 4096

func SplitMessage(text string, maxLength int) []string {
	if maxLength <= 0 {
		maxLength = DefaultMaxLength
	}
	if len(text) <= maxLength {
		return []string{text}
	}

	var chunks []string
	var current strings.Builder

	for _, line := range strings.Split(text, "\n") {
		extra := len(line) + 1
		if current.Len()+extra > maxLength {
			if current.Len() > 0 {
				chunks = append(chunks, current.String())
				current.Reset()
			}
			for len(line) > maxLength {
				chunks = append(chunks, line[:maxLength])
				line = line[maxLength:]
			}
			current.WriteString(line)
			continue
		}
		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(line)
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}
