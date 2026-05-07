#!/bin/bash

# Daily Transaction Analysis Script
# This script should be run daily (e.g., via cron) to generate AI-powered insights
# Add to crontab: 0 0 * * * /path/to/analyze_daily.sh

set -e

API_URL="${API_URL:-http://localhost:8080}"
LOG_FILE="${LOG_FILE:-/var/log/finance-analysis.log}"

echo "[$(date)] Starting daily transaction analysis..." | tee -a "$LOG_FILE"

# Fetch recent transactions
echo "Fetching transactions..." | tee -a "$LOG_FILE"
TRANSACTIONS=$(curl -s "$API_URL/api/transactions")

if [ -z "$TRANSACTIONS" ]; then
    echo "Error: Failed to fetch transactions" | tee -a "$LOG_FILE"
    exit 1
fi

# TODO: Send to AI for analysis
# This is where you would:
# 1. Format transactions for LLM
# 2. Send to Ollama/OpenAI with analysis prompt
# 3. Parse AI response
# 4. Store insights in database

echo "Analysis complete!" | tee -a "$LOG_FILE"

# Example: Call a custom endpoint to trigger AI analysis
# curl -X POST "$API_URL/api/analytics/generate" \
#   -H "Content-Type: application/json" \
#   -d "$TRANSACTIONS"

echo "[$(date)] Daily analysis finished" | tee -a "$LOG_FILE"
