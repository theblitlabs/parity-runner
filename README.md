# Parity Protocol

Parity Protocol is a decentralized compute network where runners can execute compute tasks (e.g., running a Docker file) and earn incentives in the form of tokens. Task creators can add tasks to a pool, and the first runner to complete a task successfully receives a reward.

## Quick Start

### Prerequisites

- Go 1.20 or higher
- Docker
- PostgreSQL
- Make

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

#### Using Docker

```bash
# Build the Docker image
make docker-build

# Start the application
make docker-up

# View logs
make docker-logs

# Stop the application
make docker-down
```

### Testing

Run all tests:

```bash
make test
```

## API Endpoints

### Tasks

- Create a task:

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Test Task",
    "description": "This is a test task",
    "file_url": "https://example.com/task.zip",
    "reward": 100
  }'
```

- List tasks:

```bash
curl http://localhost:8080/api/tasks
```

- Get task by ID:

```bash
curl http://localhost:8080/api/tasks/{task_id}
```

## Project Structure

```
parity-protocol/
├── cmd/                    # Entry points
├── config/                 # Configuration files
├── internal/              # Private application code
│   ├── api/               # API layer
│   ├── services/          # Business logic
│   ├── models/            # Data models
│   └── database/          # Database code
├── pkg/                   # Public packages
└── test/                 # Test files
```

## Available Make Commands

- `make run`: Run the application
- `make watch`: Run with hot reload
- `make test`: Run tests
- `make docker-up`: Start Docker containers
- `make docker-down`: Stop Docker containers
- `make migrate-up`: Run database migrations
- `make migrate-down`: Rollback migrations
- `make help`: Show all available commands

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
