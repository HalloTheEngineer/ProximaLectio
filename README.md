# ProximaLectio

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com)

A Discord bot for WebUntis school schedule integration with timetable rendering, notifications, and absence management.

## Features

- **Timetable Sync** — Automatically syncs schedules from WebUntis with visual rendering
- **Real-time Notifications** — Alerts for schedule changes, substitutions, exams, and homework
- **Statistics** — Personal academic insights and yearly progress tracking
- **Absence Management** — Track absences and generate formal excuse PDFs
- **Theming** — Customizable schedule appearance with multiple themes
- **Multi-user Support** — Works across Discord servers with shared schedule views
- **Security** — AES-256-GCM encrypted password storage

## Commands

| Command          | Description                    |
|------------------|--------------------------------|
| `/login`         | Connect your WebUntis account  |
| `/logout`        | Remove your stored credentials |
| `/today`         | View today's schedule          |
| `/tomorrow`      | View tomorrow's schedule       |
| `/week`          | Weekly overview                |
| `/room`          | Find room for a subject        |
| `/absences`      | View your absences             |
| `/exams`         | Upcoming exams                 |
| `/homework`      | View homework assignments      |
| `/stats`         | Academic statistics            |
| `/common free`   | See who's free in your server  |
| `/notifications` | Configure alerts               |
| `/theme`         | Customize schedule appearance  |
| `/excuse`        | Generate absence PDF           |

## Quick Start

### Prerequisites

- Go 1.25+ (for building from source)
- PostgreSQL 16+
- Docker & Docker Compose (for containerized deployment)

### Using Docker Compose (Recommended)

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/ProximaLectio.git
   cd ProximaLectio
   ```

2. Create a `.env` file:
   ```bash
   DISCORD_TOKEN=your_discord_bot_token
   ENCRYPTION_KEY=your-32-byte-encryption-key-here!
   DB_USER=untisuser
   DB_PASSWORD=your_secure_password
   DB_NAME=untisdb
   ```

3. Start the bot:
   ```bash
   docker-compose up -d
   ```

### Manual Build

```bash
go build -o bot .
./bot
```

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DISCORD_TOKEN` | ✅ | — | Discord bot token |
| `DB_CONNECTION_STRING` | ✅ | — | PostgreSQL connection string |
| `ENCRYPTION_KEY` | ⚠️ | `default-32-byte-encryption!` | AES encryption key (16, 24, or 32 bytes) |
| `VERBOSE` | ❌ | `0` | Enable debug logging (`1` to enable) |
| `NO_COMMAND_UPDATE` | ❌ | `0` | Skip Discord command registration |
| `HEALTH_PORT` | ❌ | `8080` | Health check server port |

> ⚠️ **Important**: Change `ENCRYPTION_KEY` in production! Stored passwords cannot be decrypted if the key is changed later.

### Health Endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /health` | Full health check (includes database ping) |
| `GET /ready` | Readiness probe for orchestrators |
| `GET /live` | Liveness probe |

## Deployment

### Coolify

1. Create a new service from Git repository
2. Set required environment variables
3. Configure `HEALTH_PORT` if 8080 is occupied
4. Deploy

### Docker

```bash
docker build -t proximalectio .
docker run -d \
  -e DISCORD_TOKEN=your_token \
  -e DB_CONNECTION_STRING=postgres://... \
  -e ENCRYPTION_KEY=your_key \
  -p 8080:8080 \
  proximalectio
```

## Architecture

```
internal/
├── cache/           # In-memory TTL caching
├── config/          # Configuration loading
├── constants/       # Application constants
├── crypto/          # AES-256-GCM password encryption
├── database/
│   ├── migrations/  # Versioned database migrations
│   ├── models/      # Data models
│   └── services/    # Business logic layer
├── discord/
│   ├── events/      # Discord event handlers
│   └── commands.go  # Slash command definitions
├── health/          # Health check HTTP server
├── untis/           # WebUntis API client with rate limiting
└── utils/           # Utility functions
```

### Service Architecture

The application follows a service-oriented architecture:

- **UntisService** — Facade coordinating all sub-services
- **UserService** — User authentication and settings
- **SchoolService** — School data management
- **SyncService** — Timetable, absence, exam, and homework synchronization
- **RenderService** — Schedule image generation
- **CleanupService** — Automated data retention

### Caching

In-memory caching reduces database load:

| Data Type | TTL | Invalidation |
|-----------|-----|--------------|
| User | 5 min | Login, Logout, Settings change |
| School | 30 min | Upsert |
| Theme | 1 hour | None (static) |
| Subjects | 2 min | After sync |
| School Search | 10 min | None |

## Development

### Prerequisites

- Go 1.25+
- PostgreSQL 16+

### Running Tests

```bash
go test ./...
```

### Database Migrations

Migrations run automatically on startup. To add a new migration:

1. Edit `internal/database/migrations/migrations.go`
2. Add a new `Migration` struct with version and SQL
3. Migrations are applied in version order

### Adding a New Command

1. Define the command in `internal/discord/commands.go`
2. Add handler case in `internal/discord/events/onCommand.go`
3. Implement the handler function

## Security

- **Password Encryption**: User WebUntis passwords are encrypted at rest using AES-256-GCM
- **Rate Limiting**: API client respects rate limits (2 RPS default)
- **No Credential Logging**: Passwords and tokens are never logged
- **Input Validation**: All user inputs are validated and sanitized

## Roadmap

- [ ] iCal/CSV schedule export
- [ ] Multi-school support per user
- [ ] Admin dashboard
- [ ] Full German localization
- [ ] Redis cache backend option

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [disgo](https://github.com/disgoorg/disgo) — Discord API library
- [canvas](https://github.com/tdewolff/canvas) — Schedule rendering
- [maroto](https://github.com/johnfercher/maroto) — PDF generation
