#!/usr/bin/env python3
import os
import sys

def generate_geometric_sequence(a, r, n):
    """Generate a geometric sequence with first term a, common ratio r, for n terms."""
    sequence = []
    for i in range(n):
        term = a * (r ** i)
        sequence.append(term)
    return sequence

def main():
    # Get task nonce from environment if available
    nonce = os.environ.get('TASK_NONCE', 'NO_NONCE_PROVIDED')
    print(f"NONCE: {nonce}")
    
    print("Geometric Number Sequence Generator")
    print("-----------------------------------")
    
    # Default values
    a = 1  # First term
    r = 2  # Common ratio
    n = 10  # Number of terms
    
    # Generate and print the sequence
    sequence = generate_geometric_sequence(a, r, n)
    print(f"Sequence with a={a}, r={r}, n={n}:")
    print(sequence)
    
    print("\nExecution successful!")

if __name__ == "__main__":
    main()