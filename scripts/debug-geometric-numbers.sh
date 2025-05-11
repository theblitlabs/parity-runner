#!/bin/bash

echo "Debugging geometric-numbers container with exit code 2..."

# Create a temporary script file for easier escaping
TMP_SCRIPT=$(mktemp)

cat > $TMP_SCRIPT << 'EOF'
import os
import sys
import traceback

try:
    print('DEBUG: Checking environment variables:')
    print(f'TASK_NONCE: {os.environ.get("TASK_NONCE", "not set")}')
    
    print('\nDEBUG: Checking if sequences.py exists:')
    if os.path.exists('/app/sequences.py'):
        print('File exists at /app/sequences.py')
        print(f'File size: {os.path.getsize("/app/sequences.py")} bytes')
        print(f'File permissions: {oct(os.stat("/app/sequences.py").st_mode)}')
    else:
        print('ERROR: File /app/sequences.py does not exist!')
        for root, dirs, files in os.walk('/app'):
            print(f'Files in {root}: {files}')
    
    print('\nDEBUG: Directory listing:')
    os.system('ls -la /app/')
    
    if os.path.exists('/app/sequences.py'):
        print('\nDEBUG: Attempting to load sequences.py:')
        with open('/app/sequences.py', 'r') as f:
            script_content = f.read()
            print(f'First 100 chars of script: {script_content[:100]}...')
        
        print('\nDEBUG: Checking for syntax errors:')
        try:
            compile(script_content, '/app/sequences.py', 'exec')
            print('No syntax errors found')
        except SyntaxError as e:
            print(f'SYNTAX ERROR: {e}')
            print(f'Error on line: {e.lineno}, column: {e.offset}')
            if e.text:
                print(f'Error line content: {e.text.strip()}')
        
        print('\nDEBUG: Attempting to execute script in try/except block:')
        try:
            exec(script_content)
            print('Script executed successfully')
        except Exception as e:
            print(f'RUNTIME ERROR: {e.__class__.__name__}: {e}')
            print(traceback.format_exc())
except Exception as e:
    print(f'DIAGNOSTIC ERROR: {e.__class__.__name__}: {e}')
    print(traceback.format_exc())
EOF

# Run the container with the script
docker run --name debug-geometric-container -e TASK_NONCE=debug123 geometric-numbers python3 -c "$(cat $TMP_SCRIPT)" || \
docker run --name debug-geometric-container -e TASK_NONCE=debug123 --entrypoint python3 geometric-numbers -c "$(cat $TMP_SCRIPT)"

# Display the container logs
echo -e "\nContainer logs:"
docker logs debug-geometric-container

# Clean up
docker rm debug-geometric-container >/dev/null 2>&1
rm -f $TMP_SCRIPT

echo -e "\nDebugging complete." 