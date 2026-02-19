# ProximaLectio

A Discord bot for WebUntis school schedule integration with notifications, timetable rendering, and absence management.

## Features

- **Timetable Sync**: Automatically syncs schedules from WebUntis
- **Visual Schedule Rendering**: Generates beautiful schedule images
- **Notifications**: Real-time alerts for schedule changes, exams, and homework
- **Absence Management**: Track and generate excuse PDFs
- **Statistics**: Personal academic insights
- **Multi-user Support**: Works across Discord servers

## Requirements

- Go 1.25+
- PostgreSQL 16+
- Docker (optional)

## Quick Start

### Environment Variables

Create a `.env` file or set these environment variables:

```bash
# Required
DISCORD_TOKEN=your_discord_bot_token
DB_CONNECTION_STRING=postgres://user:password@localhost:5432/dbname

# Optional
VERBOSE=1
NO_COMMAND_UPDATE=0
ENCRYPTION_KEY=your-32-byte-encryption-key-here!
```

### Docker Compose

```bash
docker-compose up -d
```

### Manual Build

```bash
go build -o bot .
./bot
```

## Architecture

```
internal/
├── config/          # Configuration loading
├── constants/       # Application constants
├── crypto/          # Password encryption (AES-GCM)
├── database/
│   ├── migrations/  # Database migrations
│   ├── models/      # Data models
│   └── services/    # Business logic
├── discord/
│   ├── events/      # Discord event handlers
│   └── commands.go  # Slash command definitions
├── errors/          # Error types and helpers
├── health/          # Health check endpoints
├── untis/           # WebUntis API client
└── utils/           # Utility functions
```

## Security

- Passwords are encrypted at rest using AES-256-GCM
- Encryption key should be 16, 24, or 32 bytes
- Webhook URLs are validated before storage
- Sensitive data is redacted in logs

## Health Endpoints

The bot exposes health check endpoints on port 8080:

- `GET /health` - Full health check (includes database)
- `GET /ready` - Readiness probe
- `GET /live` - Liveness probe

## Database Migrations

Migrations are run automatically on startup. The migration system tracks applied migrations in the `schema_migrations` table.

## API Retry Logic

The WebUntis API client includes:
- Configurable retry attempts (default: 3)
- Exponential backoff for transient errors
- Automatic re-authentication on 401 responses
- Recursion depth limits to prevent stack overflow

## Testing

```bash
go test ./...
```

## Commands

| Command | Description |
|---------|-------------|
| `/login` | Connect your WebUntis account |
| `/logout` | Remove your stored credentials |
| `/today` | View today's schedule |
| `/tomorrow` | View tomorrow's schedule |
| `/week` | Weekly overview |
| `/room` | Find room for a subject |
| `/absences` | View your absences |
| `/exams` | Upcoming exams |
| `/stats` | Academic statistics |
| `/notifications` | Configure alerts |
| `/theme` | Customize schedule appearance |
| `/excuse` | Generate absence PDF |

## License

MIT
