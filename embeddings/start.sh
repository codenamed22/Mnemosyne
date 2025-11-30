#!/bin/bash
# Start the CLIP embedding service

cd "$(dirname "$0")"

# Check if virtual environment exists
if [ ! -d "venv" ]; then
    echo "Creating virtual environment..."
    python3 -m venv venv
fi

# Activate virtual environment
source venv/bin/activate

# Install dependencies
echo "Installing dependencies..."
pip install -r requirements.txt -q

# Start the service
echo "Starting CLIP embedding service on http://127.0.0.1:8081"
python main.py

