---
title: Quick Start
sidebar_position: 1
---

# Quick Start

Get OpenPact running in under 5 minutes with Docker.

## Prerequisites

Before you begin, you'll need:

1. **Docker** installed on your system
   - [Get Docker](https://docs.docker.com/get-docker/)

2. **A Discord Bot Token**
   - Create a bot at the [Discord Developer Portal](https://discord.com/developers/applications)
   - Enable the "Message Content Intent" under Bot settings
   - Copy the bot token

## Run with Docker

Start OpenPact with a single command:

```bash
docker run -d \
  --name openpact \
  -v openpact-workspace:/workspace \
  -e DISCORD_TOKEN=your_discord_bot_token \
  -p 8080:8080 \
  ghcr.io/open-pact/openpact:latest
```

Replace `your_discord_bot_token` with your actual bot token.

### What This Does

- Creates a container named `openpact`
- Mounts a persistent volume for your workspace at `/workspace`
- Passes your Discord token as an environment variable
- Exposes the admin UI and health check endpoint on port 8080

## Verify It's Running

### Check Container Status

```bash
docker ps
```

You should see the `openpact` container running.

### Check Logs

```bash
docker logs openpact
```

Look for messages indicating successful startup:
- Discord connection established
- MCP server started
- Health check endpoints available

### Check Health Endpoint

```bash
curl http://localhost:8080/health
```

A healthy response looks like:

```json
{
  "status": "healthy",
  "checks": {
    "discord": "connected",
    "mcp": "ready"
  }
}
```

## Send Your First Message

1. **Invite the bot to your Discord server**
   - Go to the Discord Developer Portal
   - Navigate to OAuth2 > URL Generator
   - Select scopes: `bot`
   - Select permissions: `Send Messages`, `Read Message History`
   - Copy the generated URL and open it in your browser

2. **Message the bot**
   - Find the bot in your server
   - Send it a direct message or mention it in a channel
   - Try: "Hello! What can you do?"

## Next Steps

- **[Installation Guide](./installation)** - Docker Compose setup and building from source
- **[First Steps](./first-steps)** - Detailed Discord setup and configuration
- **[Configuration Overview](../configuration/overview)** - Customize OpenPact for your needs

## Troubleshooting

### Container Exits Immediately

Check the logs for errors:

```bash
docker logs openpact
```

Common issues:
- Invalid Discord token
- Missing required environment variables

### Bot Doesn't Respond

1. Verify the bot is online in Discord (green status indicator)
2. Check if Message Content Intent is enabled in Discord Developer Portal
3. Ensure your Discord user ID is in the allowed users list (if configured)

### Health Check Fails

```bash
curl -v http://localhost:8080/health
```

If the port isn't accessible:
- Verify the container is running: `docker ps`
- Check if port 8080 is already in use
- Try a different port: `-p 9090:8080`

## Stop and Remove

To stop OpenPact:

```bash
docker stop openpact
```

To remove the container (your data in the volume is preserved):

```bash
docker rm openpact
```

To also remove the workspace volume:

```bash
docker volume rm openpact-workspace
```
