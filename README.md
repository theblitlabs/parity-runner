# Parity Protocol

Parity Protocol is a decentralized compute network where runners can execute compute tasks (e.g., running a Docker file) and earn incentives in the form of tokens. Task creators can add tasks to a pool, and the first runner to complete a task successfully receives a reward.

## Setup & Installation

### Prerequisites

Before you begin, ensure you have the following installed:

- Go 1.23.0 or higher (using Go toolchain 1.24.0)
- Make
- Docker
  - Make sure the Docker daemon is running (`docker ps` to verify)

### Installation Steps

1. Clone the repository:

```bash
git clone https://github.com/theblitlabs/parity-runner.git
cd parity-runner
```

2. Install dependencies and development tools:

```bash
make deps
make install-lint-tools  # Install code quality tools
make install-hooks      # Install git hooks for development
```

4. Configure the application:

```bash
# Copy the example config file
cp config/config.example.yaml config/config.yaml

# Edit the config file with your settings
# See Configuration section below for details
```

5. Install the Parity Runner globally:

```bash
make install
```

### Network Participation

1. Authenticate with your private key:

```bash
parity-runner auth --private-key YOUR_PRIVATE_KEY
```

2. Stake tokens to participate in the network:

```bash
parity-runner stake --amount 10
```

3. Start running tasks:

```bash
parity-runner
```

That's it! You're now participating in the Parity Protocol network.

### Verification (Optional)

You can verify your setup with these commands:

```bash
parity-runner balance  # Check your token balance
parity-runner help    # View all available commands
```

### Development Tools

The project includes several helpful Makefile commands for development:

```bash
make build          # Build the application
make clean          # Clean build files
make deps           # Download dependencies
make fmt            # Format code using gofumpt
make imports        # Fix imports formatting
make format         # Run all formatters (gofumpt + goimports)
make lint           # Run linting
make format-lint    # Format code and run linters
make run            # Start the task runner
make stake          # Stake tokens in the network
make balance        # Check token balances
make auth           # Authenticate with the network
make install        # Install parity-runner command globally
make uninstall      # Remove parity-runner command from system
make install-lint-tools # Install formatting and linting tools
make install-hooks  # Install git hooks
make help           # Display all available commands
```

## Configuration

Create a `config.yaml` file in the `config` directory using the example provided:

```yaml
server:
  port: "8080"
  host: "localhost"
  endpoint: "/api"
  websocket:
    write_wait: 10s
    pong_wait: 60s
    max_message_size: 512

ethereum:
  rpc: "http://localhost:8545"
  chain_id: 1337
  token_address: "0x..."
  stake_wallet_address: "0x..."

runner:
  server_url: "http://localhost:8080"
  webhook_port: "8081"
  heartbeat_interval: 30s
  docker:
    memory_limit: "2g"
    cpu_limit: "1.0"
    timeout: 300
```

## CLI Commands

The CLI provides a unified interface through the `parity-runner` command:

```bash
# Show available commands and help
parity-runner help

# Authenticate with your private key
parity auth --private-key <private-key>

# Start a runner
parity-runner

# Check balance
parity-runner balance

# Stake tokens
parity-runner stake --amount <amount>
```

Each command supports the `--help` flag for detailed usage information:

```bash
parity-runner <command> --help
```

## API Documentation

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

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Install pre-commit hooks:
   ```bash
   make install-hooks
   ```
   This will install git hooks that run:
   - Code quality, security, and verification checks before each commit
   - Conventional commit message validation
4. Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification for your commit messages:

   ```
   <type>[optional scope]: <description>

   [optional body]

   [optional footer(s)]
   ```

   Valid types: feat, fix, chore, docs, style, refactor, perf, test, build, ci, revert

5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
