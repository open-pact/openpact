---
title: Calendar Integration
sidebar_position: 6
---

# Calendar Integration

OpenPact can read events from iCal calendar feeds, giving your AI assistant awareness of your schedule. This enables calendar-aware responses, reminders, and scheduling assistance.

## Overview

The calendar integration provides:

- **Event Reading**: Access upcoming events from any iCal feed
- **Multiple Calendars**: Support for multiple calendar sources
- **Flexible Queries**: Look ahead by configurable time periods
- **Standard Format**: Works with Google Calendar, Apple Calendar, Outlook, and more

## Adding iCal Feeds

Configure calendars in `openpact.yaml`:

```yaml
calendars:
  - name: Personal
    url: https://calendar.google.com/calendar/ical/your-id/basic.ics
  - name: Work
    url: https://outlook.office365.com/owa/calendar/your-id/calendar.ics
  - name: Holidays
    url: https://www.google.com/calendar/ical/en.usa%23holiday%40group.v.calendar.google.com/public/basic.ics
```

### Configuration Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `name` | string | Yes | Display name for the calendar |
| `url` | string | Yes | iCal feed URL |

### Getting Your iCal URL

#### Google Calendar

1. Open Google Calendar
2. Click the three dots next to your calendar
3. Select **Settings and sharing**
4. Scroll to **Integrate calendar**
5. Copy the **Secret address in iCal format**

:::caution Private URL
The "secret address" contains a unique token. Don't share it publicly, as anyone with the URL can read your calendar.
:::

#### Apple iCloud Calendar

1. Go to [icloud.com/calendar](https://www.icloud.com/calendar)
2. Click the share icon next to your calendar
3. Check **Public Calendar**
4. Copy the URL provided

#### Microsoft Outlook / Office 365

1. Open Outlook Calendar
2. Right-click your calendar
3. Select **Share** > **Publish this calendar**
4. Choose permissions and copy the ICS link

#### Other Calendar Services

Most calendar services support iCal exports. Look for:
- "Subscribe to calendar"
- "iCal feed"
- "Export as ICS"
- "Calendar sharing"

## Reading Events

Use the `calendar_read` tool to access events.

### Tool Usage

```json
{
  "name": "calendar_read",
  "arguments": {
    "calendar": "Personal",
    "days": 7
  }
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `calendar` | string | No | Calendar name (reads all if not specified) |
| `days` | number | No | Days to look ahead (default: 7) |

### Examples

**Read all calendars for the next week:**
```json
{
  "name": "calendar_read",
  "arguments": {}
}
```

**Read only work calendar for next 14 days:**
```json
{
  "name": "calendar_read",
  "arguments": {
    "calendar": "Work",
    "days": 14
  }
}
```

**Check today's events:**
```json
{
  "name": "calendar_read",
  "arguments": {
    "days": 1
  }
}
```

### Response Format

Events are returned with:

- **Title**: Event name
- **Start**: Start date and time
- **End**: End date and time
- **Location**: Event location (if specified)
- **Description**: Event notes (if available)
- **Calendar**: Which calendar it's from

Example response:
```json
{
  "events": [
    {
      "title": "Team Standup",
      "start": "2024-01-15T09:00:00-08:00",
      "end": "2024-01-15T09:30:00-08:00",
      "location": "Conference Room A",
      "calendar": "Work"
    },
    {
      "title": "Dentist Appointment",
      "start": "2024-01-15T14:00:00-08:00",
      "end": "2024-01-15T15:00:00-08:00",
      "location": "123 Main St",
      "calendar": "Personal"
    }
  ]
}
```

## Supported Formats

OpenPact supports the standard iCalendar format (RFC 5545), which includes:

### Event Types

- **Single Events**: One-time events
- **Recurring Events**: Daily, weekly, monthly, yearly patterns
- **All-day Events**: Events without specific times
- **Multi-day Events**: Events spanning multiple days

### Time Zones

Events are parsed with timezone awareness:
- Times are converted to a consistent format
- Timezone information is preserved
- UTC times are properly handled

### Standard Fields

Supported iCalendar fields:

| Field | Description |
|-------|-------------|
| `SUMMARY` | Event title |
| `DTSTART` | Start date/time |
| `DTEND` | End date/time |
| `LOCATION` | Event location |
| `DESCRIPTION` | Event details |
| `RRULE` | Recurrence rules |

## Use Cases

### Daily Schedule Overview

```
User: "What's on my calendar today?"

AI: Let me check your schedule.
[Uses calendar_read with days: 1]

You have 3 events today:
- 9:00 AM - Team Standup (30 min)
- 11:00 AM - 1:1 with Manager
- 3:00 PM - Code Review Session
```

### Schedule Awareness

```
User: "Am I free tomorrow afternoon?"

AI: Let me check your calendar.
[Uses calendar_read]

Tomorrow afternoon looks clear! You have a meeting at 10 AM,
but nothing scheduled after noon.
```

### Proactive Reminders

Combined with `chat_send`, the AI can send reminders through any connected chat provider:

```
User: "Remind me 15 minutes before my meetings"

AI: I'll monitor your calendar and send reminders.
[Later, uses chat_send]

"Reminder: Your meeting 'Product Review' starts in 15 minutes."
```

### Week Planning

```
User: "What does my week look like?"

AI: [Uses calendar_read with days: 7]

Here's your week:
- Monday: 3 meetings, heaviest day
- Tuesday: 1 meeting in the morning
- Wednesday: No meetings - good focus time
- Thursday: 2 meetings in the afternoon
- Friday: Team retrospective at 4 PM
```

## Limitations

### Read-Only Access

Calendar integration is read-only. You cannot:
- Create new events
- Modify existing events
- Delete events

To manage your calendar, use your calendar application directly.

### Feed Refresh

iCal feeds are fetched when requested. They are not continuously monitored. For real-time calendar changes:
- Changes may take time to appear in the feed
- Some providers cache feeds for a period

### Authentication

OpenPact supports public iCal URLs. For calendars requiring authentication:
- Use the "secret" or "private" URL if available
- Consider using a calendar proxy service

## Security Considerations

### Protecting Calendar URLs

iCal feed URLs often contain authentication tokens:

```yaml
# Keep these URLs private
calendars:
  - name: Personal
    url: https://calendar.google.com/calendar/ical/abc123xyz/basic.ics
```

- Don't share your `openpact.yaml` publicly if it contains calendar URLs
- Use environment variables for sensitive URLs:

```yaml
calendars:
  - name: Personal
    url: ${PERSONAL_CALENDAR_URL}
```

### Data Sensitivity

Calendar events may contain sensitive information:
- Meeting titles and descriptions
- Locations
- Attendee information

Be mindful of this data flowing through the AI.

## Troubleshooting

### No Events Returned

If `calendar_read` returns empty:

1. Verify the iCal URL is correct and accessible
2. Check if there are events in the requested time range
3. Test the URL in a browser or calendar app
4. Check network connectivity in the container

### Wrong Times

If event times seem off:

1. Check timezone configuration
2. Verify the calendar source timezone settings
3. Review the raw iCal feed for timezone data

### Feed Not Updating

If you don't see recent changes:

1. Wait for the calendar provider to update their feed
2. Some providers cache feeds for 5-30 minutes
3. Restart OpenPact to clear any local caching

## Related Documentation

- **[MCP Tools Reference](./mcp-tools)** - Complete tool documentation
- **[Discord Integration](./discord-integration)** - For calendar reminders
- **[Configuration Overview](../configuration/overview)** - General configuration
