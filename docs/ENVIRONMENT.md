# Environment Variables

## Quick Reference

### Required for Production
```bash
APP_ENV=production
SESSION_SECRET=your-secure-secret-key-here
```

### Common Settings
```bash
PORT=8080                    # Server port (default: 8080)
HOST=0.0.0.0                 # Server host (default: 0.0.0.0)
LOG_LEVEL=info              # debug, info, warn, error (default: debug in dev, info in prod)
```

## Full Environment Variables

### Application
| Variable | Default | Description |
|----------|---------|-------------|
| `APP_ENV` | `development` | Environment: `development` or `production` |
| `PORT` | `8080` | HTTP server port |
| `HOST` | `0.0.0.0` | Server bind address |
| `LOG_LEVEL` | `debug` (dev), `info` (prod) | Log verbosity |

### Server (Production)
| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_READ_TIMEOUT` | `30` | HTTP read timeout (seconds) |
| `SERVER_WRITE_TIMEOUT` | `30` | HTTP write timeout (seconds) |
| `SERVER_IDLE_TIMEOUT` | `120` | HTTP idle timeout (seconds) |
| `SERVER_MAX_CONNS` | `100` | Maximum concurrent connections |

### Quiz Settings
| Variable | Default | Description |
|----------|---------|-------------|
| `MAX_PLAYERS` | `100` | Maximum players per session |
| `MAX_QUESTIONS` | `50` | Maximum questions per quiz |
| `AUTO_LOAD_SAMPLE` | `true` | Auto-load sample quiz on startup |
| `QUIZ_TIMER_DEFAULT` | `30` | Default timer per question (seconds) |

### Security (Production)
| Variable | Default | Description |
|----------|---------|-------------|
| `SESSION_SECRET` | - | **Required in production!** Session signing key |
| `CORS_ORIGINS` | `*` | Allowed CORS origins (comma-separated) |
| `RATE_LIMIT_ENABLED` | `true` | Enable rate limiting |
| `RATE_LIMIT_RPS` | `100` | Rate limit (requests per second) |
| `RATE_LIMIT_BURST` | `200` | Rate limit burst size |

## Usage Examples

### Development
```bash
# Run with development defaults
./quiz-forge

# Or explicitly
APP_ENV=development LOG_LEVEL=debug ./quiz-forge
```

### Production
```bash
# Minimal production setup
APP_ENV=production \
PORT=8080 \
SESSION_SECRET=your-secure-secret-here \
./quiz-forge

# Full production configuration
APP_ENV=production \
PORT=8080 \
HOST=0.0.0.0 \
LOG_LEVEL=info \
SESSION_SECRET=your-secure-secret-here \
CORS_ORIGINS=https://yourdomain.com \
RATE_LIMIT_ENABLED=true \
RATE_LIMIT_RPS=100 \
MAX_PLAYERS=50 \
./quiz-forge
```

### Docker
```bash
# Development
docker run -p 8080:8080 quiz-forge:dev

# Production
docker run -p 8080:8080 \
  -e APP_ENV=production \
  -e SESSION_SECRET=your-secret \
  -e CORS_ORIGINS=https://yourdomain.com \
  quiz-forge:latest
```

## Environment Detection

The application automatically detects the environment:

1. If `APP_ENV=production` is set → Production mode
2. Otherwise → Development mode

### Development Mode Features
- Pretty console logging with debug info
- CORS allows all origins (`*`)
- Rate limiting disabled
- Verbose request logging
- Debug endpoints available (with `dev` build tag)

### Production Mode Features
- JSON structured logging
- CORS configured by `CORS_ORIGINS`
- Rate limiting enabled
- Panic recovery without stack traces in logs
- Production HTTP timeouts
