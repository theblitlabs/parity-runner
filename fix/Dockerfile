FROM python:3.9-slim

# Set the working directory
WORKDIR /app

# Copy the script into the container
COPY sequences.py /app/sequences.py

# Make the script executable
RUN chmod +x /app/sequences.py

# Set the entrypoint to run the script
ENTRYPOINT ["python3", "sequences.py"]