# Parity Runner

Parity Runner is a compute execution node for the PLGenesis decentralized AI and compute network. Runners execute tasks, run LLM inference, and earn token rewards for their computational contributions. This component includes built-in support for Docker containers, shell commands, and Large Language Model inference via Ollama.

## 🚀 Features

### 🤖 LLM Inference Capabilities

- **Multi-Model Support**: Supports various LLM models (Qwen, LLaMA, Mistral, etc.)
- **Ollama Integration**: Seamless integration with Ollama for local LLM execution
- **Automatic Model Management**: Downloads and manages models automatically
- **Performance Optimization**: Efficient GPU/CPU utilization for inference
- **Token Counting**: Accurate tracking of prompt and response tokens for billing

### ⚡ Compute Task Execution

- **Docker Support**: Execute arbitrary containers with resource limits
- **Shell Commands**: Run native shell scripts and commands
- **Resource Management**: CPU, memory, and timeout controls
- **Async Processing**: Non-blocking task execution with status reporting
- **Error Recovery**: Robust error handling and reporting

### 🔒 Network Integration

- **Secure Registration**: Authenticate and register with the network
- **Heartbeat Monitoring**: Regular status updates to maintain online presence
- **Webhook Processing**: Real-time task notifications from the server
- **Capability Reporting**: Automatic detection and reporting of available models

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
# Copy the sample environment file
cp .env.sample .env

# Edit the .env file with your settings
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

3. Start the runner with LLM capabilities:

```bash
parity-runner runner --config-path .env
```

This will automatically:

- Set up Ollama with default models
- Register with the server
- Start processing tasks and LLM requests
- Begin earning rewards for completed work

That's it! You're now participating in the PLGenesis network.

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

Create a `.env` file in the root directory using the sample provided (`.env.sample`):

```env
# Ethereum config
ETHEREUM_CHAIN_ID=
ETHEREUM_RPC=
ETHEREUM_STAKE_WALLET_ADDRESS=
ETHEREUM_TOKEN_ADDRESS=

# Runner config
RUNNER_DOCKER_CPU_LIMIT=
RUNNER_DOCKER_MEMORY_LIMIT=
RUNNER_DOCKER_TIMEOUT=
RUNNER_EXECUTION_TIMEOUT=
RUNNER_HEARTBEAT_INTERVAL=
RUNNER_SERVER_URL=
RUNNER_WEBHOOK_PORT=

# Server config
SERVER_ENDPOINT=
SERVER_HOST=
SERVER_PORT=
SERVER_WEBSOCKET_MAX_MESSAGE_SIZE=
SERVER_WEBSOCKET_PONG_WAIT=
SERVER_WEBSOCKET_WRITE_WAIT=
```

Example values for a local development setup:

```env
# Ethereum config
ETHEREUM_CHAIN_ID=314159  # Filecoin Calibration testnet
ETHEREUM_RPC=https://api.calibration.node.glif.io/rpc/v1  # Filecoin Calibration RPC
ETHEREUM_STAKE_WALLET_ADDRESS=0x7465e7a637f66cb7b294b856a25bc84abff1d247  # Deployed stake wallet contract
ETHEREUM_TOKEN_ADDRESS=0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0       # Deployed PRTY token contract

# Runner config
RUNNER_DOCKER_CPU_LIMIT=1.0
RUNNER_DOCKER_MEMORY_LIMIT=2g
RUNNER_DOCKER_TIMEOUT=60s        # Timeout for Docker operations (create/start/stop)
RUNNER_EXECUTION_TIMEOUT=15m     # Maximum time allowed for task execution
RUNNER_HEARTBEAT_INTERVAL=30s
RUNNER_SERVER_URL=http://localhost:8080
RUNNER_WEBHOOK_PORT=8081

# Server config
SERVER_ENDPOINT=/api
SERVER_HOST=localhost
SERVER_PORT=8080
SERVER_WEBSOCKET_MAX_MESSAGE_SIZE=512
SERVER_WEBSOCKET_PONG_WAIT=60s
SERVER_WEBSOCKET_WRITE_WAIT=10s
```

### Contract Addresses (Filecoin Calibration Testnet)

- Stake Wallet Contract: [0x7465e7a637f66cb7b294b856a25bc84abff1d247](https://filfox.info/en/address/0x7465e7a637f66cb7b294b856a25bc84abff1d247)
- PRTY Token Contract: [0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0](https://filfox.info/en/address/0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0)

You can get a free RPC endpoint for Filecoin Calibration from:

- [Glif.io](https://api.calibration.node.glif.io/rpc/v1) (Official)
- [ChainStack](https://chainstack.com/)
- [Ankr](https://ankr.com/)

You can specify a custom configuration path in three ways (in order of precedence):

1. Command line flag:

```bash
parity-runner --config-path=/path/to/.env
```

2. Environment variable:

```bash
export PARITY_CONFIG_PATH=/path/to/.env
parity-runner
```

3. Default path:
   If neither the flag nor environment variable is set, it will use `.env` in the current directory.

## CLI Commands

The CLI provides a unified interface through the `parity-runner` command:

```bash
# Show available commands and help
parity-runner help

# Authenticate with your private key
parity auth --private-key <private-key>

# Check balance
parity-runner balance

# Stake tokens
parity-runner stake --amount <amount>

# Start a runner
parity-runner

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
