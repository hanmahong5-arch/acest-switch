#!/bin/bash
# Initialize Lago with CodeSwitch billing configuration
# Run this after Lago is up and running

set -e

LAGO_API_URL="${LAGO_API_URL:-http://localhost:3001}"
LAGO_API_KEY="${LAGO_API_KEY:?LAGO_API_KEY is required}"

echo "============================================================"
echo "Initializing Lago for CodeSwitch"
echo "API URL: $LAGO_API_URL"
echo "============================================================"

# Helper function to make API calls
lago_api() {
    local method=$1
    local endpoint=$2
    local data=$3

    curl -s -X "$method" \
        "$LAGO_API_URL/api/v1/$endpoint" \
        -H "Authorization: Bearer $LAGO_API_KEY" \
        -H "Content-Type: application/json" \
        ${data:+-d "$data"}
}

echo ""
echo "1. Creating Billable Metrics..."
echo "------------------------------------------------------------"

# LLM Tokens metric
lago_api POST billable_metrics '{
    "billable_metric": {
        "name": "LLM Tokens",
        "code": "llm_tokens",
        "description": "Token consumption for LLM API calls",
        "aggregation_type": "sum_agg",
        "field_name": "tokens",
        "recurring": false
    }
}'
echo "   Created: llm_tokens"

# API Requests metric
lago_api POST billable_metrics '{
    "billable_metric": {
        "name": "API Requests",
        "code": "api_requests",
        "description": "Number of API requests",
        "aggregation_type": "count_agg",
        "recurring": false
    }
}'
echo "   Created: api_requests"

echo ""
echo "2. Creating Subscription Plans..."
echo "------------------------------------------------------------"

# Free Plan
lago_api POST plans '{
    "plan": {
        "name": "免费版",
        "code": "free",
        "interval": "monthly",
        "amount_cents": 0,
        "amount_currency": "CNY",
        "trial_period": 0,
        "pay_in_advance": false,
        "charges": [
            {
                "billable_metric_code": "llm_tokens",
                "charge_model": "standard",
                "properties": {
                    "amount": "0.001"
                }
            }
        ]
    }
}'
echo "   Created: free (¥0/month)"

# Monthly VIP Plan
lago_api POST plans '{
    "plan": {
        "name": "月度 VIP",
        "code": "monthly_vip",
        "interval": "monthly",
        "amount_cents": 4900,
        "amount_currency": "CNY",
        "trial_period": 0,
        "pay_in_advance": true,
        "charges": [
            {
                "billable_metric_code": "llm_tokens",
                "charge_model": "graduated",
                "properties": {
                    "graduated_ranges": [
                        {"from_value": 0, "to_value": 50000, "per_unit_amount": "0", "flat_amount": "0"},
                        {"from_value": 50001, "to_value": null, "per_unit_amount": "0.0009", "flat_amount": "0"}
                    ]
                }
            }
        ]
    }
}'
echo "   Created: monthly_vip (¥49/month, 50K free tokens)"

# Yearly VIP Plan
lago_api POST plans '{
    "plan": {
        "name": "年度 VIP",
        "code": "yearly_vip",
        "interval": "yearly",
        "amount_cents": 39900,
        "amount_currency": "CNY",
        "trial_period": 0,
        "pay_in_advance": true,
        "charges": [
            {
                "billable_metric_code": "llm_tokens",
                "charge_model": "graduated",
                "properties": {
                    "graduated_ranges": [
                        {"from_value": 0, "to_value": 720000, "per_unit_amount": "0", "flat_amount": "0"},
                        {"from_value": 720001, "to_value": null, "per_unit_amount": "0.0008", "flat_amount": "0"}
                    ]
                }
            }
        ]
    }
}'
echo "   Created: yearly_vip (¥399/year, 720K free tokens)"

echo ""
echo "3. Creating Coupons..."
echo "------------------------------------------------------------"

# Welcome coupon
lago_api POST coupons '{
    "coupon": {
        "name": "新用户礼包",
        "code": "WELCOME2024",
        "coupon_type": "fixed_amount",
        "amount_cents": 1000,
        "amount_currency": "CNY",
        "frequency": "once",
        "expiration": "time_limit",
        "expiration_at": "2025-12-31T23:59:59Z",
        "reusable": true,
        "limited_plans": false
    }
}'
echo "   Created: WELCOME2024 (¥10 off)"

echo ""
echo "============================================================"
echo "Lago initialization complete!"
echo "============================================================"
echo ""
echo "Plans created:"
echo "  - free:        ¥0/month"
echo "  - monthly_vip: ¥49/month (50K free tokens, 10% off after)"
echo "  - yearly_vip:  ¥399/year (720K free tokens, 20% off after)"
echo ""
echo "Billable Metrics:"
echo "  - llm_tokens:    Sum of tokens consumed"
echo "  - api_requests:  Count of API calls"
echo ""
echo "Coupons:"
echo "  - WELCOME2024:   ¥10 off for new users"
echo ""
