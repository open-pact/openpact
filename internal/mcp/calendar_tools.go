package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// CalendarConfig holds calendar configuration
type CalendarConfig struct {
	Name string // Calendar name
	URL  string // iCal feed URL
}

// RegisterCalendarTools adds calendar-related tools to the server
func RegisterCalendarTools(s *Server, calendars []CalendarConfig) {
	s.RegisterTool(calendarReadTool(calendars))
}

// Event represents a calendar event
type Event struct {
	Summary     string
	Description string
	Location    string
	Start       time.Time
	End         time.Time
	AllDay      bool
}

// calendarReadTool creates a tool for reading calendar events
func calendarReadTool(calendars []CalendarConfig) *Tool {
	return &Tool{
		Name:        "calendar_read",
		Description: "Read upcoming events from configured calendars",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"calendar": map[string]interface{}{
					"type":        "string",
					"description": "Calendar name to read (optional, reads all if not specified)",
				},
				"days": map[string]interface{}{
					"type":        "integer",
					"description": "Number of days to look ahead (default: 7)",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			calName, _ := args["calendar"].(string)
			daysFloat, _ := args["days"].(float64)
			days := int(daysFloat)
			if days <= 0 {
				days = 7
			}

			// Determine which calendars to read
			var calsToRead []CalendarConfig
			if calName != "" {
				for _, c := range calendars {
					if strings.EqualFold(c.Name, calName) {
						calsToRead = append(calsToRead, c)
						break
					}
				}
				if len(calsToRead) == 0 {
					return nil, fmt.Errorf("calendar '%s' not found", calName)
				}
			} else {
				calsToRead = calendars
			}

			if len(calsToRead) == 0 {
				return "No calendars configured", nil
			}

			// Fetch and parse events
			now := time.Now()
			endDate := now.AddDate(0, 0, days)
			var allEvents []Event

			for _, cal := range calsToRead {
				events, err := fetchCalendarEvents(ctx, cal.URL, now, endDate)
				if err != nil {
					return nil, fmt.Errorf("failed to fetch '%s': %w", cal.Name, err)
				}
				allEvents = append(allEvents, events...)
			}

			// Sort by start time
			sort.Slice(allEvents, func(i, j int) bool {
				return allEvents[i].Start.Before(allEvents[j].Start)
			})

			// Format output
			if len(allEvents) == 0 {
				return fmt.Sprintf("No events in the next %d days", days), nil
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Events in the next %d days:\n\n", days))

			currentDate := ""
			for _, e := range allEvents {
				dateStr := e.Start.Format("Monday, January 2")
				if dateStr != currentDate {
					currentDate = dateStr
					sb.WriteString(fmt.Sprintf("## %s\n", dateStr))
				}

				if e.AllDay {
					sb.WriteString(fmt.Sprintf("- [All day] %s\n", e.Summary))
				} else {
					timeStr := e.Start.Format("15:04")
					endStr := e.End.Format("15:04")
					sb.WriteString(fmt.Sprintf("- [%s-%s] %s\n", timeStr, endStr, e.Summary))
				}

				if e.Location != "" {
					sb.WriteString(fmt.Sprintf("  Location: %s\n", e.Location))
				}
			}

			return sb.String(), nil
		},
	}
}

// fetchCalendarEvents fetches and parses an iCal feed
func fetchCalendarEvents(ctx context.Context, url string, start, end time.Time) ([]Event, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return parseICalEvents(resp.Body, start, end)
}

// parseICalEvents parses iCal data and returns events in the given range
func parseICalEvents(r io.Reader, start, end time.Time) ([]Event, error) {
	var events []Event
	var currentEvent *Event
	var inEvent bool

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		// Handle line continuations
		for scanner.Scan() {
			next := scanner.Text()
			if strings.HasPrefix(next, " ") || strings.HasPrefix(next, "\t") {
				line += strings.TrimLeft(next, " \t")
			} else {
				// Put it back conceptually by processing line first
				// then handling this line in next iteration
				if strings.HasPrefix(line, "BEGIN:VEVENT") {
					inEvent = true
					currentEvent = &Event{}
				} else if strings.HasPrefix(line, "END:VEVENT") && inEvent && currentEvent != nil {
					// Check if event is in range
					if !currentEvent.Start.IsZero() &&
						currentEvent.Start.Before(end) &&
						(currentEvent.End.IsZero() || currentEvent.End.After(start)) {
						events = append(events, *currentEvent)
					}
					inEvent = false
					currentEvent = nil
				} else if inEvent && currentEvent != nil {
					parseEventLine(line, currentEvent)
				}

				line = next
				continue
			}
		}

		// Process the final accumulated line
		if strings.HasPrefix(line, "BEGIN:VEVENT") {
			inEvent = true
			currentEvent = &Event{}
		} else if strings.HasPrefix(line, "END:VEVENT") && inEvent && currentEvent != nil {
			if !currentEvent.Start.IsZero() &&
				currentEvent.Start.Before(end) &&
				(currentEvent.End.IsZero() || currentEvent.End.After(start)) {
				events = append(events, *currentEvent)
			}
			inEvent = false
			currentEvent = nil
		} else if inEvent && currentEvent != nil {
			parseEventLine(line, currentEvent)
		}
	}

	return events, scanner.Err()
}

// parseEventLine parses a single iCal line into an event
func parseEventLine(line string, event *Event) {
	if idx := strings.Index(line, ":"); idx > 0 {
		key := line[:idx]
		value := line[idx+1:]

		// Handle parameters in key (e.g., DTSTART;VALUE=DATE:20260203)
		keyParts := strings.Split(key, ";")
		baseKey := keyParts[0]
		isDateOnly := false
		for _, p := range keyParts[1:] {
			if strings.EqualFold(p, "VALUE=DATE") {
				isDateOnly = true
			}
		}

		switch baseKey {
		case "SUMMARY":
			event.Summary = unescapeIcal(value)
		case "DESCRIPTION":
			event.Description = unescapeIcal(value)
		case "LOCATION":
			event.Location = unescapeIcal(value)
		case "DTSTART":
			t, allDay := parseIcalTime(value, isDateOnly)
			event.Start = t
			event.AllDay = allDay
		case "DTEND":
			t, _ := parseIcalTime(value, isDateOnly)
			event.End = t
		}
	}
}

// parseIcalTime parses an iCal datetime string
func parseIcalTime(s string, isDateOnly bool) (time.Time, bool) {
	s = strings.TrimSpace(s)

	if isDateOnly || len(s) == 8 {
		// Date only: YYYYMMDD
		t, err := time.Parse("20060102", s)
		if err == nil {
			return t, true
		}
	}

	// Try various datetime formats
	formats := []string{
		"20060102T150405Z",     // UTC
		"20060102T150405",      // Local
		"2006-01-02T15:04:05Z", // ISO with dashes
		"2006-01-02T15:04:05",
	}

	for _, fmt := range formats {
		t, err := time.Parse(fmt, s)
		if err == nil {
			return t, false
		}
	}

	return time.Time{}, false
}

// unescapeIcal unescapes iCal string values
func unescapeIcal(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\,", ",")
	s = strings.ReplaceAll(s, "\\;", ";")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}
