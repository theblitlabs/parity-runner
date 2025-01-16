# Parity Protocol

Parity Protocol is a decentralized compute network where runners can execute compute tasks (e.g., running a Docker file) and earn incentives in the form of tokens. Task creators can add tasks to a pool, and the first runner to complete a task successfully receives a reward.

## Quick Start

### Prerequisites

- Go 1.20 or higher
- PostgreSQL
- Make
- Docker (optional, for containerized database)

### Installation

1. Clone the repository:

```bash
git clone https://github.com/virajbhartiya/parity-protocol.git
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

### Running the Application

#### Development Mode (with hot reload)

```bash
make watch
```

#### Standard Mode

```bash
make run
```

### Testing

```bash
# Run tests with coverage
make test

# Run tests with verbose output
make test-verbose
```

## API Endpoints

All API endpoints are prefixed with `/api/v1`. For example:

### Tasks

- Create a task:

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
-H "Content-Type: application/json" \
-d '{
  "title": "Docker Test Task",
  "description": "Run a simple Docker container",
  "type": "docker",
  "reward": 100,
  "config": {
    "command": ["echo", "Hello from Docker!"]
  },
  "environment": {
    "type": "docker",
    "config": {
      "image": "alpine:latest",
      "command": ["echo", "Hello from Docker!"],
      "env": ["FOO=bar"],
      "workdir": "/app",
      "volumes": {
        "/tmp": "/container-tmp"
      }
    }
  }
}'
```

- List all tasks:

```bash
curl http://localhost:8080/api/v1/tasks
```

- Get task by ID:

```bash
curl http://localhost:8080/api/v1/tasks/{id}
```

- Get task reward:

```bash
curl http://localhost:8080/api/v1/tasks/{id}/reward
```

| Method | Endpoint                  | Description       |
| ------ | ------------------------- | ----------------- |
| GET    | /api/v1/tasks             | List all tasks    |
| POST   | /api/v1/tasks             | Create a new task |
| GET    | /api/v1/tasks/{id}        | Get task by ID    |
| GET    | /api/v1/tasks/{id}/reward | Get task reward   |

## Project Structure

```
parity-protocol/
├── cmd/                    # Application entry points
│   ├── migrate/           # Database migration tool
│   └── server/            # Main application server
├── config/                # Configuration files
│   └── config.yaml       # Application configuration
├── internal/              # Private application code
│   ├── api/              # API layer
│   ├── services/         # Business logic
│   ├── models/           # Data models
│   └── database/         # Database code
├── pkg/                   # Public packages
└── test/                 # Test files
```

## Available Make Commands

### Core Commands

- `make build`: Build the application
- `make run`: Run the application
- `make watch`: Run with hot reload (requires air)
- `make install-air`: Install air for hot reloading
- `make clean`: Clean build files and test artifacts

### Testing Commands

- `make test`: Run tests with coverage
- `make test-verbose`: Run tests with verbose output
- `make setup-coverage`: Create coverage directory

### Database Commands

- `make migrate-up`: Run database migrations up
- `make migrate-down`: Run database migrations down

### Development Commands

- `make deps`: Download and tidy dependencies
- `make fmt`: Format code

### Installation Commands

- `make install`: Install parity command globally
- `make uninstall`: Remove parity command from system

### Helper Commands

- `make help`: Display help screen with all available commands

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
