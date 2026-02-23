# github_notify.star - Example Starlark script for OpenPact
# @description: Check GitHub notifications and summarize
# @author: OpenPact
# @version: 1.0.0
# @secrets: GITHUB_TOKEN

def get_notifications(all=False):
    """
    Fetch GitHub notifications

    Args:
        all: Include read notifications (default: False)

    Returns:
        List of notification summaries
    """
    token = secrets.get("GITHUB_TOKEN")
    if token == None:
        return {"error": "GITHUB_TOKEN not configured"}

    url = "https://api.github.com/notifications"
    if all:
        url = url + "?all=true"

    resp = http.get(url, headers={
        "Authorization": format("Bearer %s", token),
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28"
    })

    if resp["status"] != 200:
        return {"error": "API request failed", "status": resp["status"]}

    data = json.decode(resp["body"])
    notifications = []

    for n in data:
        notifications.append({
            "id": n["id"],
            "reason": n["reason"],
            "title": n["subject"]["title"],
            "type": n["subject"]["type"],
            "repo": n["repository"]["full_name"],
            "updated_at": n["updated_at"],
            "unread": n["unread"]
        })

    return {
        "count": len(notifications),
        "notifications": notifications
    }

def mark_read(notification_id):
    """
    Mark a notification as read

    Args:
        notification_id: The notification ID to mark as read

    Returns:
        Success status
    """
    token = secrets.get("GITHUB_TOKEN")
    if token == None:
        return {"error": "GITHUB_TOKEN not configured"}

    url = format("https://api.github.com/notifications/threads/%s", notification_id)

    resp = http.post(url, body="", headers={
        "Authorization": format("Bearer %s", token),
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28"
    }, content_type="application/json")

    # GitHub returns 205 Reset Content on success
    if resp["status"] != 205 and resp["status"] != 200:
        return {"error": "Failed to mark as read", "status": resp["status"]}

    return {"success": True, "notification_id": notification_id}

# Default execution
result = get_notifications()
