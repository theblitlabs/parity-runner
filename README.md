# Parity Runner

Parity Runner is a compute execution node for the PLGenesis decentralized AI and compute network. Runners execute tasks, run LLM inference, perform federated learning training, and earn token rewards for their computational contributions. This component includes built-in support for Docker containers, shell commands, Large Language Model inference via Ollama, and federated learning with comprehensive data partitioning strategies.

## 🚀 Features

### 🤖 LLM Inference Capabilities

- **Multi-Model Support**: Supports various LLM models (Qwen, LLaMA, Mistral, etc.)
- **Ollama Integration**: Seamless integration with Ollama for local LLM execution
- **Automatic Model Management**: Downloads and manages models automatically
- **Performance Optimization**: Efficient GPU/CPU utilization for inference
- **Token Counting**: Accurate tracking of prompt and response tokens for billing

### 🧠 Federated Learning Capabilities

- **Neural Network Training**: Support for multi-layer neural networks with configurable architectures
- **Linear Regression**: Built-in linear regression training capabilities
- **Distributed Random Forest**: Complete random forest implementation with federated learning support
  - **Bootstrap Sampling**: Configurable subsample ratios with bagging
  - **Random Feature Selection**: Configurable number of features per split
  - **Decision Tree Building**: Full binary tree construction with Gini impurity
  - **Out-of-Bag (OOB) Scoring**: Automatic model validation using unused samples
  - **Feature Importance**: Calculates and tracks feature importance across all trees
  - **Model Aggregation**: Combines feature importance and tree statistics across nodes
  - **Privacy-Preserving**: No raw data sharing between participants in distributed training
- **Data Partitioning**: Advanced partitioning strategies for truly distributed FL:
  - **Random (IID)**: Uniform random distribution
  - **Stratified**: Maintains class distribution across participants
  - **Sequential**: Consecutive data splits
  - **Non-IID**: Dirichlet distribution for realistic data heterogeneity
  - **Label Skew**: Each participant gets subset of classes with optional overlap
- **IPFS/Blockchain Integration**: Automatic data loading from decentralized storage
- **Mandatory IPFS Storage**: All datasets must be stored on IPFS and accessed via CID
  - **Supported Formats**: CSV and JSON data formats with automatic validation
  - **Multiple Gateways**: Uses multiple IPFS gateways for reliable data retrieval
- **Numerical Stability**: Comprehensive NaN protection and safe weight initialization
- **Model Aggregation**: Automatic submission of both weights and gradients to server
- **Requirements Validation**: All training parameters must be explicitly provided (no defaults)

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
git clone https://github.com/virajbhartiya/parity-runner.git
cd parity-runner
```

2. Install dependencies and development tools:

```bash
make deps
make install-lint-tools  # Install code quality tools
make install-hooks      # Install git hooks for development
```

3. Configure the application:

```bash
# Copy the sample environment file
cp .env.sample .env

# Edit the .env file with your settings
# See Configuration section below for details
```

4. Install the Parity Runner globally:

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

3. Start the runner with LLM and FL capabilities:

```bash
parity-runner runner --config-path .env
```

This will automatically:

- Set up Ollama with default models
- Register with the server for task execution
- Start processing compute tasks, LLM requests, and FL training
- Begin earning rewards for completed work

That's it! You're now participating in the PLGenesis network and can receive federated learning training tasks.

## 🌐 Tunnel Support (NAT/Firewall Bypass)

PLGenesis Runner includes **automatic tunneling** to expose webhook endpoints through NAT/firewall using **bore.pub**. This enables runners behind routers or firewalls to participate without manual port forwarding.

### Quick Tunnel Setup

#### Option 1: Auto-Setup (Recommended)

```bash
# Automatically enables tunneling and starts runner
make run-tunnel
```

#### Option 2: Manual Setup

```bash
# 1. Install tunnel support
make install-tunnel

# 2. Enable in .env
echo "RUNNER_TUNNEL_ENABLED=true" >> .env

# 3. Start runner normally
parity-runner runner --config-path .env
```

### Tunnel Features

- ✅ **Automatic bore.pub integration** - Free public tunnel service
- ✅ **Auto-installation** - Installs bore CLI automatically if needed
- ✅ **Zero configuration** - Works out of the box with sensible defaults
- ✅ **Self-hostable** - Support for private bore servers with authentication
- ✅ **Robust error handling** - Automatic reconnection and health monitoring

### Tunnel Configuration

Add these to your `.env` file:

```env
# Tunnel Configuration
RUNNER_TUNNEL_ENABLED=true
RUNNER_TUNNEL_TYPE=bore          # bore, ngrok, local, custom
RUNNER_TUNNEL_SERVER_URL=bore.pub # Default: bore.pub (free)
RUNNER_TUNNEL_PORT=0             # 0 for random port
RUNNER_TUNNEL_SECRET=            # For private servers
```

### How It Works

```
PLGenesis Server → bore.pub → Your Runner (behind NAT)
     ↓                ↓           ↓
   Tasks          Tunneled    Local Webhook
                  Traffic     Processing
```

1. Runner creates tunnel to bore.pub
2. Gets public URL (e.g., `http://bore.pub:35429/webhook`)
3. Registers this URL with PLGenesis server
4. Server sends tasks to public URL
5. bore.pub forwards to local webhook

### Commands

```bash
make install-tunnel  # Install bore CLI
make test-tunnel     # Test tunnel functionality
make run-tunnel      # Start runner with auto-tunnel
```

📖 **Detailed Documentation**: See [TUNNEL_README.md](TUNNEL_README.md) for advanced configuration, self-hosting, and troubleshooting.

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
make run-tunnel     # Start runner with auto-tunnel setup
make install-tunnel # Install bore CLI for tunneling
make test-tunnel    # Test tunnel functionality
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
# Blockchain config
BLOCKCHAIN_CHAIN_ID=
BLOCKCHAIN_RPC=
BLOCKCHAIN_STAKE_WALLET_ADDRESS=
BLOCKCHAIN_TOKEN_ADDRESS=
BLOCKCHAIN_TOKEN_SYMBOL=
BLOCKCHAIN_TOKEN_NAME=
BLOCKCHAIN_NETWORK_NAME=

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
# Blockchain config
BLOCKCHAIN_CHAIN_ID=1  # Chain ID (1 for Ethereum, 137 for Polygon, etc.)
BLOCKCHAIN_RPC=https://mainnet.infura.io/v3/YOUR_PROJECT_ID  # RPC endpoint URL
BLOCKCHAIN_STAKE_WALLET_ADDRESS=0x1234567890123456789012345678901234567890  # Deployed stake wallet contract
BLOCKCHAIN_TOKEN_ADDRESS=0xabcdefabcdefabcdefabcdefabcdefabcdefabcd       # Deployed token contract
BLOCKCHAIN_TOKEN_SYMBOL=PRTY  # Token symbol (e.g., PRTY, USDC, ETH)
BLOCKCHAIN_TOKEN_NAME=Parity Token  # Token name
BLOCKCHAIN_NETWORK_NAME=Ethereum  # Network name

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

### Contract Addresses

- Stake Wallet Contract: `0x1234567890123456789012345678901234567890` (example)
- Token Contract: `0xabcdefabcdefabcdefabcdefabcdefabcdefabcd` (example)

You can get a free RPC endpoint for various blockchains from:

- [Infura](https://infura.io/) (Ethereum, Polygon, etc.)
- [Alchemy](https://alchemy.com/) (Ethereum, Polygon, etc.)
- [QuickNode](https://quicknode.com/) (Multiple chains)
- [ChainStack](https://chainstack.com/) (Multiple chains)
- [Ankr](https://ankr.com/) (Multiple chains)

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

## Federated Learning

The parity-runner provides comprehensive federated learning capabilities with strict requirements validation.

### Key Capabilities

#### 🎯 Requirements-Based Training

- **No Default Values**: All training parameters must be provided by the server task
- **Strict Validation**: Comprehensive parameter validation with clear error messages
- **Configuration Required**: Model architecture must be specified in task configuration

#### 🔢 Model Support

- **Neural Networks**: Multi-layer perceptrons with configurable architecture
- **Linear Regression**: Support for regression tasks
- **Extensible**: Easy to add new model types

#### 📊 Data Partitioning

Automatic data partitioning based on server coordination:

1. **Random (IID)**: `strategy: "random"`

   - Uniform random distribution of data
   - Suitable for homogeneous scenarios

2. **Stratified**: `strategy: "stratified"`

   - Maintains class distribution across participants
   - Good for balanced federated learning

3. **Sequential**: `strategy: "sequential"`

   - Consecutive data splits
   - Useful for time-series or ordered data

4. **Non-IID**: `strategy: "non_iid"`

   - Dirichlet distribution for realistic data heterogeneity
   - Controlled by alpha parameter (lower = more skewed)

5. **Label Skew**: `strategy: "label_skew"`
   - Each participant gets subset of classes
   - Optional overlap between participants

#### 🛡️ Numerical Stability

- **NaN Protection**: Multiple layers of NaN detection and prevention
- **Safe Weight Initialization**: Enhanced Xavier/Glorot initialization
- **Learning Rate Validation**: Automatic bounds checking
- **Input Data Validation**: Pre-training data quality checks
- **Output Sanitization**: Final cleanup before JSON serialization

### FL Task Processing

When a runner receives an FL training task:

1. **Task Validation**: Validates all required parameters are present
2. **Data Loading**: Downloads and loads data from IPFS CID
3. **Data Partitioning**: Applies assigned partition strategy and index
4. **Model Training**: Performs local training with specified parameters
5. **Weight Extraction**: Extracts both weights and gradients
6. **Result Submission**: Submits training results to server

### Example FL Task Configuration

```json
{
  "session_id": "uuid",
  "round_id": "uuid",
  "model_type": "neural_network",
  "dataset_cid": "QmYourDatasetCID",
  "data_format": "csv",
  "model_config": {
    "input_size": 784,
    "hidden_size": 128,
    "output_size": 10
  },
  "partition_config": {
    "strategy": "non_iid",
    "total_parts": 3,
    "part_index": 0,
    "alpha": 0.5,
    "min_samples": 100,
    "overlap_ratio": 0.0
  },
  "train_config": {
    "epochs": 5,
    "batch_size": 32,
    "learning_rate": 0.001
  }
}
```

### Random Forest Configuration

Configure distributed random forest training through federated learning sessions:

```json
{
  "session_id": "rf_session_1",
  "round_id": "round_1",
  "model_type": "random_forest",
  "dataset_cid": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
  "data_format": "csv",
  "model_config": {
    "num_trees": 100,
    "max_depth": 10,
    "min_samples_split": 2,
    "min_samples_leaf": 1,
    "max_features": 0,
    "subsample": 1.0,
    "random_state": 42,
    "bootstrap_samples": true,
    "oob_score": true,
    "num_classes": 0
  },
  "train_config": {
    "epochs": 1,
    "batch_size": 32,
    "learning_rate": 0.01
  },
  "partition_config": {
    "strategy": "random",
    "total_parts": 3,
    "part_index": 0,
    "min_samples": 10
  },
  "output_format": "json"
}
```

#### Random Forest Parameters

- **num_trees**: Number of trees in the forest (default: 100)
- **max_depth**: Maximum depth of each tree (default: 10)
- **min_samples_split**: Minimum samples required to split (default: 2)
- **min_samples_leaf**: Minimum samples in leaf nodes (default: 1)
- **max_features**: Features per split (0 = sqrt(total))
- **subsample**: Fraction of samples for each tree (default: 1.0)
- **bootstrap_samples**: Enable bootstrap sampling (default: true)
- **oob_score**: Calculate out-of-bag scores (default: true)
- **num_classes**: Number of target classes (0 = auto-detect from IPFS data)

#### IPFS Dataset Requirements

All datasets must be stored on IPFS and accessed via Content ID (CID):

**Supported Formats:**

- **CSV**: First row optional headers, last column contains labels
- **JSON**: `{"features": [[...]], "labels": [...]}`

**Data Validation:**

- Feature dimensions must be consistent across samples
- No NaN/Inf values allowed
- Automatic class detection from label data
- Minimum data quality requirements enforced

### Error Handling

The FL system provides comprehensive error messages:

- **Missing Parameters**: Clear indication of required parameters
- **Invalid Values**: Specific validation error messages
- **Data Issues**: Detailed data quality error reports
- **Training Failures**: Comprehensive debugging information

## CLI Commands

The CLI provides a unified interface through the `parity-runner` command:

```bash
# Show available commands and help
parity-runner help

# Authenticate with your private key
parity-runner auth --private-key <private-key>

# Check balance
parity-runner balance

# Stake tokens
parity-runner stake --amount <amount>

# Start the runner (handles all task types including FL)
parity-runner runner
```

Each command supports the `--help` flag for detailed usage information:

```bash
parity-runner <command> --help
```

## API Documentation

Runners interact with various server endpoints. Below are the main API endpoints available:

### Federated Learning Endpoints

| Method | Endpoint                                 | Description          | Runner Usage             |
| ------ | ---------------------------------------- | -------------------- | ------------------------ |
| POST   | /api/v1/federated-learning/model-updates | Submit model updates | Automatic after training |
| GET    | /api/v1/federated-learning/sessions/{id} | Get session details  | Task validation          |

### LLM Endpoints

| Method | Endpoint                | Description                        |
| ------ | ----------------------- | ---------------------------------- |
| GET    | `/api/llm/models`       | List all available LLM models      |
| POST   | `/api/llm/prompts`      | Submit a prompt for LLM processing |
| GET    | `/api/llm/prompts/{id}` | Get prompt status and response     |
| GET    | `/api/llm/prompts`      | List recent prompts                |

### Task Endpoints

| Method | Endpoint               | Description      |
| ------ | ---------------------- | ---------------- |
| POST   | /api/tasks             | Create task      |
| GET    | /api/tasks             | List tasks       |
| GET    | /api/tasks/{id}        | Get task details |
| GET    | /api/tasks/{id}/reward | Get task reward  |
| GET    | /api/tasks/{id}/status | Get task status  |
| GET    | /api/tasks/{id}/logs   | Get task logs    |

### Runner Endpoints

| Method | Endpoint                         | Description                 |
| ------ | -------------------------------- | --------------------------- |
| POST   | /api/runners                     | Register runner             |
| POST   | /api/runners/heartbeat           | Send heartbeat              |
| GET    | /api/runners/tasks/available     | List available tasks        |
| POST   | /api/runners/tasks/{id}/start    | Start task                  |
| POST   | /api/runners/tasks/{id}/complete | Complete task               |
| POST   | /api/runners/webhooks            | Register webhook endpoint   |
| DELETE | /api/runners/webhooks/{id}       | Unregister webhook endpoint |

### Storage Endpoints

| Method | Endpoint                    | Description                  |
| ------ | --------------------------- | ---------------------------- |
| POST   | /api/storage/upload         | Upload file to IPFS |
| GET    | /api/storage/download/{cid} | Download file by CID         |
| GET    | /api/storage/info/{cid}     | Get file information         |
| POST   | /api/storage/pin/{cid}      | Pin file to IPFS             |

### Health & Status Endpoints

| Method | Endpoint    | Description   |
| ------ | ----------- | ------------- |
| GET    | /api/health | Health check  |
| GET    | /api/status | System status |

## Troubleshooting

### Common Issues

1. **Federated Learning Issues**

   - **Training parameter errors**: Ensure server provides all required parameters
   - **Data loading failures**: Check IPFS connectivity and CID validity
   - **Partition errors**: Verify partition configuration matches strategy requirements
   - **NaN values in training**: Check input data quality and learning rate values

2. **Docker Issues**

   - **Docker daemon not running**: Start Docker service
   - **Permission issues**: Ensure user is in docker group
   - **Resource limits**: Adjust memory/CPU limits in configuration

3. **Network Issues**

   - **Connection failures**: Check server URL and network connectivity
   - **Authentication issues**: Verify private key and staking status
   - **Webhook port conflicts**: Ensure webhook port is available

4. **LLM Issues**
   - **Ollama connection failures**: Ensure Ollama is installed and running
   - **Model download issues**: Check internet connectivity and disk space
   - **GPU memory issues**: Adjust model selection based on available resources

### Error Examples

**FL Parameter Error**:

```
training configuration is incomplete: learning_rate is required
```

**Solution**: Ensure the FL session was created with all required training parameters

**Data Partition Error**:

```
alpha parameter must be positive for non-IID partitioning, got 0.000000
```

**Solution**: Check that the FL session was created with appropriate alpha value for non_iid strategy

**Model Configuration Error**:

```
hidden_size is required in neural network configuration
```

**Solution**: Ensure the FL session includes complete model configuration

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
