package mcp

import (
	"strings"
	"testing"
	"time"
)

func TestParseIcalTime(t *testing.T) {
	tests := []struct {
		input      string
		isDateOnly bool
		wantAllDay bool
		wantYear   int
		wantMonth  time.Month
		wantDay    int
	}{
		{"20260203", true, true, 2026, time.February, 3},
		{"20260203", false, true, 2026, time.February, 3}, // 8 chars implies date
		{"20260203T140000Z", false, false, 2026, time.February, 3},
		{"20260203T140000", false, false, 2026, time.February, 3},
	}

	for _, tt := range tests {
		got, allDay := parseIcalTime(tt.input, tt.isDateOnly)

		if allDay != tt.wantAllDay {
			t.Errorf("parseIcalTime(%s, %v) allDay = %v, want %v",
				tt.input, tt.isDateOnly, allDay, tt.wantAllDay)
		}

		if got.Year() != tt.wantYear || got.Month() != tt.wantMonth || got.Day() != tt.wantDay {
			t.Errorf("parseIcalTime(%s, %v) = %v, want %d-%02d-%02d",
				tt.input, tt.isDateOnly, got, tt.wantYear, tt.wantMonth, tt.wantDay)
		}
	}
}

func TestUnescapeIcal(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello\\nWorld", "Hello\nWorld"},
		{"Test\\, with comma", "Test, with comma"},
		{"Semi\\;colon", "Semi;colon"},
		{"Back\\\\slash", "Back\\slash"},
		{"No escapes", "No escapes"},
	}

	for _, tt := range tests {
		got := unescapeIcal(tt.input)
		if got != tt.want {
			t.Errorf("unescapeIcal(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseICalEvents(t *testing.T) {
	ical := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
SUMMARY:Test Meeting
DTSTART:20260203T140000Z
DTEND:20260203T150000Z
LOCATION:Conference Room
END:VEVENT
BEGIN:VEVENT
SUMMARY:All Day Event
DTSTART;VALUE=DATE:20260204
END:VEVENT
BEGIN:VEVENT
SUMMARY:Past Event
DTSTART:20260101T100000Z
DTEND:20260101T110000Z
END:VEVENT
END:VCALENDAR`

	// Parse events from Feb 2-10, 2026
	start := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)

	events, err := parseICalEvents(strings.NewReader(ical), start, end)
	if err != nil {
		t.Fatalf("parseICalEvents error: %v", err)
	}

	// Should have 2 events (Test Meeting and All Day Event, not Past Event)
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
		for i, e := range events {
			t.Logf("Event %d: %s at %v", i, e.Summary, e.Start)
		}
	}

	// Check first event
	found := false
	for _, e := range events {
		if e.Summary == "Test Meeting" {
			found = true
			if e.Location != "Conference Room" {
				t.Errorf("expected location 'Conference Room', got '%s'", e.Location)
			}
			if e.AllDay {
				t.Error("expected AllDay=false for Test Meeting")
			}
		}
	}
	if !found {
		t.Error("Test Meeting not found in parsed events")
	}

	// Check all-day event
	found = false
	for _, e := range events {
		if e.Summary == "All Day Event" {
			found = true
			if !e.AllDay {
				t.Error("expected AllDay=true for All Day Event")
			}
		}
	}
	if !found {
		t.Error("All Day Event not found in parsed events")
	}
}

func TestParseEventLine(t *testing.T) {
	event := &Event{}

	parseEventLine("SUMMARY:Test Event", event)
	if event.Summary != "Test Event" {
		t.Errorf("expected Summary 'Test Event', got '%s'", event.Summary)
	}

	parseEventLine("LOCATION:Room 101", event)
	if event.Location != "Room 101" {
		t.Errorf("expected Location 'Room 101', got '%s'", event.Location)
	}

	parseEventLine("DESCRIPTION:A test\\ndescription", event)
	if event.Description != "A test\ndescription" {
		t.Errorf("expected unescaped description, got '%s'", event.Description)
	}
}

func TestCalendarReadToolNoCalendars(t *testing.T) {
	tool := calendarReadTool(nil)

	result, err := tool.Handler(nil, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.(string), "No calendars") {
		t.Errorf("expected 'No calendars' message, got: %v", result)
	}
}

func TestCalendarReadToolNotFound(t *testing.T) {
	calendars := []CalendarConfig{
		{Name: "Work", URL: "http://example.com/work.ics"},
	}
	tool := calendarReadTool(calendars)

	_, err := tool.Handler(nil, map[string]interface{}{
		"calendar": "Personal",
	})

	if err == nil {
		t.Error("expected error for non-existent calendar")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestRegisterCalendarTools(t *testing.T) {
	s := NewServer(nil, nil)
	calendars := []CalendarConfig{
		{Name: "Work", URL: "http://example.com/work.ics"},
	}

	RegisterCalendarTools(s, calendars)

	tools := s.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "calendar_read" {
		t.Errorf("expected 'calendar_read' tool, got '%s'", tools[0].Name)
	}
}
