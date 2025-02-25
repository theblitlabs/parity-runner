# Parity Protocol

Parity Protocol is a decentralized compute network where runners can execute compute tasks (e.g., running a Docker file) and earn incentives in the form of tokens. Task creators can add tasks to a pool, and the first runner to complete a task successfully receives a reward.

## Quick Start

### Prerequisites

- Go 1.22.7 or higher (using Go toolchain 1.23.4)
- PostgreSQL
- Make
- Docker (optional, for containerized database)

### Installation

1. Clone the repository:

```bash
git clone https://github.com/theblitlabs/parity-protocol.git
cd parity-protocol
```

2. Install dependencies:

```bash
make deps
```

3. Start PostgreSQL (if using Docker):

```bash
# Remove existing container if it exists
docker rm -f parity-db || true

# Start new PostgreSQL container
docker run --name parity-db -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres
```

4. Run database migrations:

```bash
make migrate-up
```

### Development

The project includes several helpful Makefile commands for development:

```bash
make build          # Build the application
make run           # Run the application
make test          # Run tests with coverage
make clean         # Clean build files
make deps          # Download dependencies
make fmt           # Format code
make watch         # Run with hot reload (requires air)
make install       # Install parity command globally
make uninstall     # Remove parity command from system
make help          # Display all available commands
```

For hot reloading during development:

```bash
# Install air (required for hot reloading)
make install-air

# Run with hot reload
make watch
```

### Configuration

Create a `config.yaml` file in the `config` directory using the example provided:

```yaml
ethereum:
  rpc: "http://localhost:8545"
  chain_id: 1337
  token_address: "0x..."
  stake_wallet_address: "0x..."

server:
  host: "localhost"
  port: "8080"
  endpoint: "/api"

database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "postgres"
  name: "parity"
  sslmode: "disable"
```

### CLI Commands

The CLI provides a unified interface through the `parity` command:

```bash
# Show available commands and help
parity help

# Authenticate with your private key
parity auth --private-key <private-key>

# Start the server
parity server

# Start a runner
parity runner

# Check balance
parity balance

# Stake tokens
parity stake --amount <amount>

# Start chain proxy
parity chain

# Run database migrations
parity migrate
```

Each command supports the `--help` flag for detailed usage information:

```bash
parity <command> --help
```

### Project Structure

```
parity-protocol/
├── cmd/                    # Application entry points
│   ├── cli/               # CLI commands
│   │   ├── auth.go       # Authentication command
│   │   ├── balance.go    # Balance checking
│   │   ├── chain.go      # Chain proxy
│   │   ├── migrate.go    # Database migrations
│   │   ├── root.go       # Root command
│   │   ├── runner.go     # Runner command
│   │   ├── server.go     # Server command
│   │   └── stake.go      # Staking command
│   └── main.go           # Main application entry
├── config/                # Configuration files
│   └── config.yaml       # Application configuration
├── internal/              # Private application code
│   ├── api/              # API layer
│   │   ├── handlers/     # Request handlers
│   │   ├── middleware/   # HTTP middleware
│   │   └── router.go     # API routing
│   ├── config/           # Configuration handling
│   ├── database/         # Database layer
│   │   └── repositories/ # Data access
│   ├── models/           # Data models
│   ├── runner/           # Runner implementation
│   └── services/         # Business logic
├── pkg/                   # Public packages
│   ├── database/         # Database utilities
│   ├── device/           # Device management
│   ├── keystore/         # Key management
│   ├── logger/           # Logging utilities
│   ├── stakewallet/      # Stake wallet interface
│   └── wallet/           # Wallet operations
├── test/                 # Integration tests
│   └── cli/              # CLI tests
└── coverage/             # Test coverage reports

```

### Testing

```bash
# Run all tests with coverage
make test

# Coverage report will be generated in coverage/coverage.html
```

### API Documentation

#### Task Creator Endpoints

| Method | Endpoint               | Description      |
| ------ | ---------------------- | ---------------- |
| POST   | /api/tasks             | Create task      |
| GET    | /api/tasks             | List tasks       |
| GET    | /api/tasks/{id}        | Get task details |
| GET    | /api/tasks/{id}/reward | Get task reward  |

#### Runner Endpoints

| Method | Endpoint                         | Description                 |
| ------ | -------------------------------- | --------------------------- |
| GET    | /api/runners/tasks/available     | List available tasks        |
| POST   | /api/runners/tasks/{id}/start    | Start task                  |
| POST   | /api/runners/tasks/{id}/complete | Complete task               |
| POST   | /api/runners/webhooks            | Register webhook endpoint   |
| DELETE | /api/runners/webhooks/{id}       | Unregister webhook endpoint |

### Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
