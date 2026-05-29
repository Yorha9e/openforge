# OpenForge

AI-driven end-to-end full-stack development workbench.

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 20+
- PostgreSQL 15+
- Docker (optional)

### Configuration

1. **Copy example configuration files:**
   ```bash
   # For development (local machine)
   cp config/profiles/minimal.yaml.example config/profiles/minimal.yaml
   
   # For Docker Compose development
   cp config/profiles/docker-dev.yaml.example config/profiles/docker-dev.yaml
   ```

2. **Set environment variables:**
   ```bash
   # Database
   export DB_PASSWORD=your_database_password
   
   # JWT
   export JWT_SECRET=your_jwt_secret_at_least_32_chars
   
   # User passwords (generate with: htpasswd -nbBC 10 "" your_password | cut -d: -f2)
   export ADMIN_PASSWORD_HASH=your_admin_password_hash
   export PM_PASSWORD_HASH=your_pm_password_hash
   
   # LLM API Keys
   export ANTHROPIC_AUTH_TOKEN=your_anthropic_api_key
   ```

3. **Edit configuration file:**
   Update `config/profiles/minimal.yaml` (or `docker-dev.yaml`) with your specific values.

### Running

**Local development:**
```bash
# Start database
docker compose up postgres -d

# Run migrations
go run cmd/openforge/main.go migrate up

# Start server
go run cmd/server/main.go

# Start frontend (in another terminal)
cd frontend && npm install && npm run dev
```

**Docker Compose:**
```bash
# Start all services
docker compose up -d

# View logs
docker compose logs -f
```

## Project Structure

```
openforge/
├── cmd/                       # Main applications
│   ├── server/               # API server
│   └── openforge/            # CLI tool
├── config/                   # Configuration files
│   ├── profiles/            # Capability profiles (minimal/standard/enterprise)
│   ├── prompts/             # LLM prompts
│   └── skills/              # Skill definitions
├── deployments/              # Deployment configurations
├── frontend/                # React frontend
├── internal/                # Go backend code
│   ├── adapter/            # External service adapters
│   ├── agent/              # AI agent implementation
│   ├── auth/               # Authentication
│   ├── llm/                # LLM integration
│   ├── observability/      # Monitoring and logging
│   ├── pipeline/           # Pipeline execution
│   ├── server/             # HTTP handlers
│   └── shared/             # Shared utilities
├── migrations/              # Database migrations
├── nodejs-io/              # Node.js I/O service
└── proto/                  # Protocol buffer definitions
```

## Configuration Profiles

OpenForge supports three configuration profiles:

- **minimal**: Single-machine development, small teams (<10 people)
- **standard**: Single-AZ K8s, mid-size teams (50-200 people)
- **enterprise**: Multi-AZ, regulated industries

See `config/profiles/` for example configurations.

## Security

### Sensitive Information

**Never commit these files to version control:**
- `.env` files
- Configuration files with hardcoded passwords
- API keys or tokens
- Private keys or certificates

**Use environment variables for:**
- Database passwords
- JWT secrets
- API keys
- User password hashes

### Generating Password Hashes

```bash
# Install htpasswd (if not available)
# macOS: brew install httpd
# Ubuntu: sudo apt-get install apache2-utils

# Generate hash
htpasswd -nbBC 10 "" your_password | cut -d: -f2
```

## Development

### Running Tests

```bash
# Go tests
go test ./...

# Frontend tests
cd frontend && npm test

# Integration tests
go test -tags=integration ./...
```

### Building

```bash
# Build Go binaries
go build -o bin/server cmd/server/main.go
go build -o bin/openforge cmd/openforge/main.go

# Build frontend
cd frontend && npm run build
```

## License

[Add your license here]
