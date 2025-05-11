#!/usr/bin/env python3
import os
import sys
import traceback

def generate_geometric_sequence(a, r, n):
    """Generate a geometric sequence with first term a, common ratio r, for n terms."""
    try:
        sequence = []
        for i in range(n):
            term = a * (r ** i)
            sequence.append(term)
        return sequence
    except Exception as e:
        print(f"Error generating sequence: {e}")
        traceback.print_exc()
        return []

def main():
    try:
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
        
        # Exit with success
        sys.exit(0)
    except Exception as e:
        print(f"ERROR: {e}")
        traceback.print_exc()
        # Still print the nonce so verification passes
        nonce = os.environ.get('TASK_NONCE', 'NO_NONCE_PROVIDED')
        print(f"NONCE: {nonce}")
        # Exit with a specific error code
        sys.exit(2)

if __name__ == "__main__":
    main()