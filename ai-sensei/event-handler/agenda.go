package main

import (
	"fmt"
	"strings"
)

// detectStepCompletion checks if the AI response contains a step completion signal
func detectStepCompletion(response string, currentStep int) bool {
	// Slack太字マーカー(*) を除去してからパターンマッチ
	cleaned := strings.ReplaceAll(response, "*", "")
	stepStr := fmt.Sprintf("%d", currentStep)

	patterns := []string{
		fmt.Sprintf("ステップ%sが完了しました", stepStr),
		fmt.Sprintf("ステップ%sが完了", stepStr),
		fmt.Sprintf("ステップ%s完了", stepStr),
	}

	for _, pattern := range patterns {
		if strings.Contains(cleaned, pattern) {
			return true
		}
	}

	return false
}

// extractAgenda extracts the agenda section from the lecture text and returns structured items
func extractAgenda(lecture string) []AgendaItem {
	// Look for "今回のアジェンダ:" or "アジェンダ:"
	agendaStart := -1
	if idx := strings.Index(lecture, "今回のアジェンダ:"); idx != -1 {
		agendaStart = idx
	} else if idx := strings.Index(lecture, "アジェンダ:"); idx != -1 {
		agendaStart = idx
	}

	if agendaStart == -1 {
		return nil // No agenda found
	}

	// Find the end of agenda (look for "まず" or double newline)
	remaining := lecture[agendaStart:]
	lines := strings.Split(remaining, "\n")

	var agendaItems []AgendaItem
	stepNumber := 0

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Stop if we hit "まず" or empty line indicating end of agenda
		if strings.HasPrefix(line, "まず") || (line == "" && i > 1) {
			break
		}

		var description string
		matched := false

		// Parse numbered items like "1. 基本概念の理解"
		if len(line) > 2 && line[0] >= '1' && line[0] <= '9' {
			parts := strings.SplitN(line, ".", 2)
			if len(parts) == 2 {
				description = strings.TrimSpace(parts[1])
				matched = true
			}
		}

		// Also parse bullet points like "• 基本概念の理解" or "- 基本概念の理解"
		if !matched && len(line) > 2 {
			if strings.HasPrefix(line, "• ") || strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				description = strings.TrimSpace(line[2:]) // Skip the bullet and space
				matched = true
			}
		}

		if matched && description != "" {
			stepNumber++
			agendaItems = append(agendaItems, AgendaItem{
				StepNumber:  stepNumber,
				Description: description,
				Completed:   false,
			})
		}
	}

	return agendaItems
}
