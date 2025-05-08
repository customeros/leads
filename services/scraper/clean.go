package scraper

import (
	"regexp"
	"strings"
)

type DocumentSection struct {
	Heading   string
	Content   []string
	LinkCount int
	TextCount int
	ListCount int
}

// PreprocessMarkdownWebpage extracts core content from Markdown by removing navigation,
// menus, headers, footers while preserving the main content
func processMarkdownWebpage(markdownContent string) string {
	// Phase 1: Remove navigation sections
	cleanText := removeNavigationSections(markdownContent)

	// Phase 2: Remove individual navigation elements
	cleanText = removeNavigationElements(cleanText)

	// Phase 3: Clean up and format the result
	return formatOutput(cleanText)
}

func removeNavigationSections(text string) string {
	lines := strings.Split(text, "\n")

	// First pass: parse into sections and remove link-heavy ones
	cleanSections := removeNavigationHeavySections(parseDocumentSections(lines))

	// Second pass: remove list-heavy sections and navigation markers
	finalSections := removeListsAndNavigationMarkers(cleanSections)

	return renderDocument(finalSections)
}

// parseDocumentSections divides text into logical sections
func parseDocumentSections(lines []string) []DocumentSection {
	var sections []DocumentSection
	var current DocumentSection

	for _, line := range lines {
		if isHeading(line) || isHorizontalRule(line) {
			// Save previous section if it exists
			if current.Heading != "" || len(current.Content) > 0 {
				sections = append(sections, current)
			}

			// Start a new section
			current = DocumentSection{Heading: line}
		} else {
			// Analyze line content
			trimmedLine := strings.TrimSpace(line)
			if isMarkdownLink(line) || isHtmlLink(line) || isLinkDefinition(line) {
				current.LinkCount++
			} else if isLinkListItem(line) {
				current.ListCount++
			} else if len(trimmedLine) > 0 {
				current.TextCount++
			}

			current.Content = append(current.Content, line)
		}
	}

	// Add the final section
	if current.Heading != "" || len(current.Content) > 0 {
		sections = append(sections, current)
	}

	return sections
}

// removeNavigationHeavySections filters out sections with more links than content
func removeNavigationHeavySections(sections []DocumentSection) []DocumentSection {
	var filtered []DocumentSection

	for _, section := range sections {
		if section.LinkCount <= section.TextCount {
			filtered = append(filtered, section)
		}
	}

	return filtered
}

// removeListsAndNavigationMarkers removes list-heavy sections and cleans navigation markers
func removeListsAndNavigationMarkers(sections []DocumentSection) []DocumentSection {
	var result []DocumentSection

	for _, section := range sections {
		// Skip list-heavy sections
		isListHeavy := section.ListCount > 0 &&
			(section.TextCount == 0 || section.ListCount > section.TextCount*3)

		if isListHeavy {
			continue
		}

		// Filter out navigation markers from content
		cleanContent := []string{}
		for _, line := range section.Content {
			if !isNavigationMarker(line) && !isLinkSection(line) && !isLinkListItem(line) {
				cleanContent = append(cleanContent, line)
			}
		}

		// Only keep sections with content after cleaning
		if section.Heading != "" || len(cleanContent) > 0 {
			cleanSection := section
			cleanSection.Content = cleanContent
			result = append(result, cleanSection)
		}
	}

	return result
}

// renderDocument joins sections back into text
func renderDocument(sections []DocumentSection) string {
	var lines []string

	for _, section := range sections {
		if section.Heading != "" {
			lines = append(lines, section.Heading)
		}
		lines = append(lines, section.Content...)
	}

	return strings.Join(lines, "\n")
}

// Helper functions to identify different elements

func isHeading(line string) bool {
	headingRegex := regexp.MustCompile(`^#{1,6}\s+.+$|^.+\n[=\-]+$`)
	return headingRegex.MatchString(line)
}

func isHorizontalRule(line string) bool {
	hrRegex := regexp.MustCompile(`^(\*{3,}|-{3,}|_{3,})$`)
	return hrRegex.MatchString(strings.TrimSpace(line))
}

func isMarkdownLink(line string) bool {
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	return linkRegex.MatchString(line)
}

func isHtmlLink(line string) bool {
	htmlLinkRegex := regexp.MustCompile(`<a\s+[^>]*>[^<]*<\/a>`)
	return htmlLinkRegex.MatchString(line)
}

func isLinkDefinition(line string) bool {
	linkDefRegex := regexp.MustCompile(`^\s*\[[^\]]+\]:\s*http.+$`)
	return linkDefRegex.MatchString(line)
}

func isListItem(line string) bool {
	listItemRegex := regexp.MustCompile(`^\s*[\*\-+]\s+.+$|^\s*\d+\.\s+.+$`)
	return listItemRegex.MatchString(line)
}

func isLinkListItem(line string) bool {
	linkListItemRegex := regexp.MustCompile(`^\s*[\*\-+]\s+\[.+\]\(.+\).*$|^\s*\d+\.\s+\[.+\]\(.+\).*$`)
	return linkListItemRegex.MatchString(line)
}

func isNavigationMarker(line string) bool {
	navMarkers := []string{
		"Navigation", "Menu", "Links", "Buttons", "Toggle navigation",
		"Copyright", "All rights reserved", "Follow us on", "Find us on",
		"Links/Buttons:", "Social media", "Back to", "Skip to",
	}

	lowered := strings.ToLower(line)
	for _, marker := range navMarkers {
		if strings.Contains(lowered, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func isLinkSection(line string) bool {
	linkSectionRegex := regexp.MustCompile(`^(Links|Nav|Menu|Navigation|Footer)(\W|$)`)
	return linkSectionRegex.MatchString(strings.TrimSpace(line))
}

// removeNavigationElements applies regex filters to remove specific navigation elements
func removeNavigationElements(text string) string {
	// Patterns to identify and remove
	patterns := []*regexp.Regexp{
		// Links - only remove if they're a list of links with no surrounding context
		regexp.MustCompile(`(?m)^\s*\* \[[^\]]+\]\([^)]+\)\s*$`), // List item links
		regexp.MustCompile(`(?m)^\s*- \[[^\]]+\]\([^)]+\)\s*$`),  // List item links with dash
		regexp.MustCompile(`(?m)^\s*\+ \[[^\]]+\]\([^)]+\)\s*$`), // List item links with plus

		// Navigation lines and sections
		regexp.MustCompile(`(?i)toggle navigation`),           // Bootstrap toggle nav
		regexp.MustCompile(`(?i)copyright ©\d{4}.*`),          // Copyright notices
		regexp.MustCompile(`(?i)all rights reserved.*`),       // Rights reserved
		regexp.MustCompile(`(?i)(follow us on|find us on).*`), // Social media sections
		regexp.MustCompile(`(?m)^Links/Buttons:$.*?^$`),       // Links section headers
	}

	// Apply each pattern
	for _, pattern := range patterns {
		text = pattern.ReplaceAllString(text, "")
	}

	return text
}

// formatOutput cleans up the extracted text for better readability
func formatOutput(text string) string {
	// Clean up excessive blank lines (more than 2 consecutive)
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")

	// Fix spacing around punctuation
	text = regexp.MustCompile(`\s+([.,;:!?])`).ReplaceAllString(text, "$1")

	// Keep important headings (= and - underlined headings)
	text = regexp.MustCompile(`(?m)^([^\n]+)\n[=]+\s*$`).ReplaceAllString(text, "$1\n")
	text = regexp.MustCompile(`(?m)^([^\n]+)\n[-]+\s*$`).ReplaceAllString(text, "$1\n")

	// Trim leading/trailing whitespace
	return strings.TrimSpace(text)
}
