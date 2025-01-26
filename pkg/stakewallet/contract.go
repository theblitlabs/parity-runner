package stakewallet

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// StakeWalletContract is the Go binding of the StakeWallet contract
type StakeWalletContract struct {
	address common.Address
	backend bind.ContractBackend
	abi     abi.ABI
}

// Your ABI as a string constant
const StakeWalletABI = `[
    {
      "inputs": [
        {
          "internalType": "address",
          "name": "_tokenAddress",
          "type": "address"
        }
      ],
      "stateMutability": "nonpayable",
      "type": "constructor"
    },
    {
      "inputs": [
        {
          "internalType": "address",
          "name": "owner",
          "type": "address"
        }
      ],
      "name": "OwnableInvalidOwner",
      "type": "error"
    },
    {
      "inputs": [
        {
          "internalType": "address",
          "name": "account",
          "type": "address"
        }
      ],
      "name": "OwnableUnauthorizedAccount",
      "type": "error"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "string",
          "name": "deviceId",
          "type": "string"
        },
        {
          "indexed": true,
          "internalType": "address",
          "name": "recipient",
          "type": "address"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "ownerAmount",
          "type": "uint256"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "recipientAmount",
          "type": "uint256"
        }
      ],
      "name": "Distributed",
      "type": "event"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "address",
          "name": "previousOwner",
          "type": "address"
        },
        {
          "indexed": true,
          "internalType": "address",
          "name": "newOwner",
          "type": "address"
        }
      ],
      "name": "OwnershipTransferred",
      "type": "event"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "string",
          "name": "deviceId",
          "type": "string"
        },
        {
          "indexed": true,
          "internalType": "address",
          "name": "staker",
          "type": "address"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "amount",
          "type": "uint256"
        }
      ],
      "name": "Staked",
      "type": "event"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "_deviceId",
          "type": "string"
        },
        {
          "internalType": "address",
          "name": "_recipient",
          "type": "address"
        },
        {
          "internalType": "uint256",
          "name": "_ownerAmount",
          "type": "uint256"
        },
        {
          "internalType": "uint256",
          "name": "_recipientAmount",
          "type": "uint256"
        }
      ],
      "name": "distributeStake",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "_deviceId",
          "type": "string"
        }
      ],
      "name": "getBalanceByDeviceId",
      "outputs": [
        {
          "internalType": "uint256",
          "name": "balance",
          "type": "uint256"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "_deviceId",
          "type": "string"
        }
      ],
      "name": "getStakeInfo",
      "outputs": [
        {
          "internalType": "uint256",
          "name": "amount",
          "type": "uint256"
        },
        {
          "internalType": "address",
          "name": "staker",
          "type": "address"
        },
        {
          "internalType": "bool",
          "name": "exists",
          "type": "bool"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [],
      "name": "owner",
      "outputs": [
        {
          "internalType": "address",
          "name": "",
          "type": "address"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [],
      "name": "renounceOwnership",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "uint256",
          "name": "_amount",
          "type": "uint256"
        },
        {
          "internalType": "string",
          "name": "_deviceId",
          "type": "string"
        }
      ],
      "name": "stake",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "",
          "type": "string"
        }
      ],
      "name": "stakes",
      "outputs": [
        {
          "internalType": "uint256",
          "name": "amount",
          "type": "uint256"
        },
        {
          "internalType": "address",
          "name": "staker",
          "type": "address"
        },
        {
          "internalType": "bool",
          "name": "exists",
          "type": "bool"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [],
      "name": "token",
      "outputs": [
        {
          "internalType": "contract IERC20",
          "name": "",
          "type": "address"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "address",
          "name": "newOwner",
          "type": "address"
        }
      ],
      "name": "transferOwnership",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    }
  ],`

// NewStakeWalletContract creates a new instance of the contract bindings
func NewStakeWalletContract(address common.Address, backend bind.ContractBackend) (*StakeWalletContract, error) {
	contractABI, err := abi.JSON(strings.NewReader(StakeWalletABI))
	if err != nil {
		return nil, err
	}

	return &StakeWalletContract{
		address: address,
		backend: backend,
		abi:     contractABI,
	}, nil
}

// GetStakeInfo retrieves stake information for a given device ID
func (c *StakeWalletContract) GetStakeInfo(opts *bind.CallOpts, deviceID string) (StakeInfo, error) {
	var out []interface{}
	// Convert deviceID to bytes32 for contract call
	deviceIDBytes := crypto.Keccak256Hash([]byte(deviceID))

	err := bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Call(opts, &out, "getStakeInfo", deviceIDBytes)
	if err != nil {
		return StakeInfo{}, err
	}

	return StakeInfo{
		Amount:   abi.ConvertType(out[0], new(big.Int)).(*big.Int),
		DeviceID: deviceID, // Keep original device ID
		Exists:   *abi.ConvertType(out[2], new(bool)).(*bool),
	}, nil
}

// Stake tokens with device ID
func (c *StakeWalletContract) Stake(opts *bind.TransactOpts, amount *big.Int, deviceID string) (*types.Transaction, error) {
	return bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Transact(opts, "stake", amount, deviceID)
}

// DistributeStake distributes stake between user and recipient
func (c *StakeWalletContract) DistributeStake(opts *bind.TransactOpts, deviceID string, recipient common.Address, ownerAmount *big.Int, recipientAmount *big.Int) (*types.Transaction, error) {
	contract := bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend)
	return contract.Transact(opts, "distributeStake", deviceID, recipient, ownerAmount, recipientAmount)
}

// Owner returns the contract owner
func (c *StakeWalletContract) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Call(opts, &out, "owner")
	if err != nil {
		return common.Address{}, err
	}
	return *abi.ConvertType(out[0], new(common.Address)).(*common.Address), nil
}

// Token returns the staking token address
func (c *StakeWalletContract) Token(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Call(opts, &out, "token")
	if err != nil {
		return common.Address{}, err
	}
	return *abi.ConvertType(out[0], new(common.Address)).(*common.Address), nil
}

// GetBalanceByDeviceID retrieves balance for a device ID
func (c *StakeWalletContract) GetBalanceByDeviceID(opts *bind.CallOpts, deviceID string) (*big.Int, error) {
	var out []interface{}
	err := bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Call(opts, &out, "getBalanceByDeviceId", deviceID)
	if err != nil {
		return nil, err
	}
	return abi.ConvertType(out[0], new(big.Int)).(*big.Int), nil
}

// GetUsersByDeviceId retrieves users for a device ID
func (c *StakeWalletContract) GetUsersByDeviceId(opts *bind.CallOpts, deviceID string) ([]common.Address, error) {
	var out []interface{}
	err := bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Call(opts, &out, "getUsersByDeviceId", deviceID)
	if err != nil {
		return nil, err
	}
	return *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address), nil
}

// CheckStakeExists checks if a user has staked tokens
func (c *StakeWalletContract) CheckStakeExists(opts *bind.CallOpts, user common.Address) (bool, error) {
	stakeInfo, err := c.GetStakeInfo(opts, user.String())
	if err != nil {
		return false, err
	}
	return stakeInfo.Exists, nil
}
