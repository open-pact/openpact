---
sidebar_position: 6
title: Examples
description: Real-world Starlark script examples
---

# Starlark Script Examples

This page provides complete, working examples of Starlark scripts for common use cases.

## Weather API Script

Fetch current weather data from WeatherAPI.com.

```python
# @description: Get current weather for a city
# @author: OpenPact Team
# @version: 1.0.0
# @secrets: WEATHER_API_KEY

def get_weather(city):
    """
    Fetch current weather conditions for a city.

    Args:
        city: City name (e.g., "London") or coordinates (e.g., "51.5,-0.1")

    Returns:
        Dict with weather data or error message
    """
    api_key = secrets.get("WEATHER_API_KEY")
    if not api_key:
        return {"error": "WEATHER_API_KEY not configured"}

    url = format(
        "https://api.weatherapi.com/v1/current.json?key=%s&q=%s&aqi=no",
        api_key,
        city
    )

    resp = http.get(url)
    if resp["status"] != 200:
        return {
            "error": "Weather API request failed",
            "status": resp["status"]
        }

    data = json.decode(resp["body"])

    return {
        "city": data["location"]["name"],
        "region": data["location"]["region"],
        "country": data["location"]["country"],
        "local_time": data["location"]["localtime"],
        "temp_c": data["current"]["temp_c"],
        "temp_f": data["current"]["temp_f"],
        "feels_like_c": data["current"]["feelslike_c"],
        "feels_like_f": data["current"]["feelslike_f"],
        "condition": data["current"]["condition"]["text"],
        "humidity": data["current"]["humidity"],
        "wind_kph": data["current"]["wind_kph"],
        "wind_mph": data["current"]["wind_mph"],
        "wind_dir": data["current"]["wind_dir"],
        "uv_index": data["current"]["uv"]
    }

def get_forecast(city, days):
    """
    Fetch weather forecast for a city.

    Args:
        city: City name
        days: Number of days (1-10)

    Returns:
        Dict with forecast data
    """
    api_key = secrets.get("WEATHER_API_KEY")
    if not api_key:
        return {"error": "WEATHER_API_KEY not configured"}

    if days < 1 or days > 10:
        return {"error": "Days must be between 1 and 10"}

    url = format(
        "https://api.weatherapi.com/v1/forecast.json?key=%s&q=%s&days=%d&aqi=no",
        api_key,
        city,
        days
    )

    resp = http.get(url)
    if resp["status"] != 200:
        return {"error": "Forecast API request failed", "status": resp["status"]}

    data = json.decode(resp["body"])
    forecasts = []

    for day in data["forecast"]["forecastday"]:
        forecasts.append({
            "date": day["date"],
            "max_temp_c": day["day"]["maxtemp_c"],
            "min_temp_c": day["day"]["mintemp_c"],
            "condition": day["day"]["condition"]["text"],
            "chance_of_rain": day["day"]["daily_chance_of_rain"],
            "chance_of_snow": day["day"]["daily_chance_of_snow"]
        })

    return {
        "city": data["location"]["name"],
        "country": data["location"]["country"],
        "forecast": forecasts
    }

# Default execution
result = get_weather("London")
```

**Usage:**

```
Tool: script_run
Args: {"name": "weather", "function": "get_weather", "args": ["Tokyo"]}
```

---

## Stock Price Script

Fetch stock quotes from Alpha Vantage.

```python
# @description: Get stock price and market data
# @author: OpenPact Team
# @version: 1.0.0
# @secrets: ALPHA_VANTAGE_API_KEY

def get_quote(symbol):
    """
    Get current stock quote for a symbol.

    Args:
        symbol: Stock ticker symbol (e.g., "AAPL", "GOOGL")

    Returns:
        Dict with stock data or error message
    """
    api_key = secrets.get("ALPHA_VANTAGE_API_KEY")
    if not api_key:
        return {"error": "ALPHA_VANTAGE_API_KEY not configured"}

    # Validate symbol
    symbol = symbol.upper().strip()
    if not symbol or len(symbol) > 10:
        return {"error": "Invalid stock symbol"}

    url = format(
        "https://www.alphavantage.co/query?function=GLOBAL_QUOTE&symbol=%s&apikey=%s",
        symbol,
        api_key
    )

    resp = http.get(url)
    if resp["status"] != 200:
        return {"error": "API request failed", "status": resp["status"]}

    data = json.decode(resp["body"])

    if "Global Quote" not in data or not data["Global Quote"]:
        return {"error": "Symbol not found or API limit reached", "symbol": symbol}

    quote = data["Global Quote"]

    return {
        "symbol": quote["01. symbol"],
        "price": float(quote["05. price"]),
        "open": float(quote["02. open"]),
        "high": float(quote["03. high"]),
        "low": float(quote["04. low"]),
        "volume": int(quote["06. volume"]),
        "previous_close": float(quote["08. previous close"]),
        "change": float(quote["09. change"]),
        "change_percent": quote["10. change percent"],
        "latest_trading_day": quote["07. latest trading day"],
        "fetched_at": time.now()
    }

def get_multiple_quotes(symbols):
    """
    Get quotes for multiple symbols.

    Args:
        symbols: List of stock ticker symbols

    Returns:
        Dict with quotes for each symbol
    """
    results = {}

    for symbol in symbols:
        # Add small delay between requests to avoid rate limiting
        if len(results) > 0:
            time.sleep(1)

        results[symbol] = get_quote(symbol)

    return {"quotes": results, "count": len(results)}

# Default execution
result = get_quote("AAPL")
```

**Usage:**

```
Tool: script_run
Args: {"name": "stocks", "function": "get_quote", "args": ["MSFT"]}
```

---

## Notification Script

Send notifications via webhooks (Slack, Discord, etc.).

```python
# @description: Send notifications to various platforms
# @author: OpenPact Team
# @version: 1.0.0
# @secrets: SLACK_WEBHOOK_URL, DISCORD_WEBHOOK_URL

def send_slack(message, channel=None):
    """
    Send a message to Slack via webhook.

    Args:
        message: The message text to send
        channel: Optional channel override

    Returns:
        Dict with success status
    """
    webhook_url = secrets.get("SLACK_WEBHOOK_URL")
    if not webhook_url:
        return {"error": "SLACK_WEBHOOK_URL not configured"}

    payload = {"text": message}
    if channel:
        payload["channel"] = channel

    resp = http.post(
        webhook_url,
        body=json.encode(payload),
        headers={"Content-Type": "application/json"}
    )

    if resp["status"] != 200:
        return {
            "success": False,
            "error": "Slack webhook failed",
            "status": resp["status"]
        }

    return {
        "success": True,
        "platform": "slack",
        "sent_at": time.now()
    }

def send_discord(message, username=None):
    """
    Send a message to Discord via webhook.

    Args:
        message: The message content to send
        username: Optional username override for the webhook

    Returns:
        Dict with success status
    """
    webhook_url = secrets.get("DISCORD_WEBHOOK_URL")
    if not webhook_url:
        return {"error": "DISCORD_WEBHOOK_URL not configured"}

    payload = {"content": message}
    if username:
        payload["username"] = username

    resp = http.post(
        webhook_url,
        body=json.encode(payload),
        headers={"Content-Type": "application/json"}
    )

    # Discord returns 204 No Content on success
    if resp["status"] not in [200, 204]:
        return {
            "success": False,
            "error": "Discord webhook failed",
            "status": resp["status"]
        }

    return {
        "success": True,
        "platform": "discord",
        "sent_at": time.now()
    }

def send_generic_webhook(url, data, headers=None):
    """
    Send data to a generic webhook URL.

    Args:
        url: The webhook URL (must be https)
        data: Dict of data to send as JSON
        headers: Optional additional headers

    Returns:
        Dict with response status
    """
    if not url.startswith("https://"):
        return {"error": "Webhook URL must use HTTPS"}

    request_headers = {"Content-Type": "application/json"}
    if headers:
        for key, value in headers.items():
            request_headers[key] = value

    resp = http.post(
        url,
        body=json.encode(data),
        headers=request_headers
    )

    return {
        "success": resp["status"] >= 200 and resp["status"] < 300,
        "status": resp["status"],
        "sent_at": time.now()
    }

# Default execution - test Slack
result = send_slack("Test notification from OpenPact")
```

**Usage:**

```
Tool: script_run
Args: {"name": "notification", "function": "send_slack", "args": ["Server deployment complete!"]}
```

---

## Data Transformation Script

Transform and aggregate data from multiple sources.

```python
# @description: Data transformation utilities
# @author: OpenPact Team
# @version: 1.0.0
# @secrets: none

def parse_csv(csv_string, has_header=True):
    """
    Parse a CSV string into a list of dicts or lists.

    Args:
        csv_string: The CSV data as a string
        has_header: Whether the first row is a header

    Returns:
        List of dicts (if has_header) or list of lists
    """
    lines = csv_string.strip().split("\n")
    if not lines:
        return {"error": "Empty CSV data"}

    result = []
    header = None

    for i, line in enumerate(lines):
        # Simple CSV parsing (doesn't handle quoted commas)
        values = [v.strip() for v in line.split(",")]

        if i == 0 and has_header:
            header = values
        elif header:
            row = {}
            for j, key in enumerate(header):
                if j < len(values):
                    row[key] = values[j]
                else:
                    row[key] = ""
            result.append(row)
        else:
            result.append(values)

    return {"data": result, "row_count": len(result)}

def aggregate_numbers(data, key):
    """
    Calculate statistics for a numeric field.

    Args:
        data: List of dicts
        key: The key to aggregate

    Returns:
        Dict with sum, avg, min, max, count
    """
    values = []
    for item in data:
        if key in item:
            try:
                values.append(float(item[key]))
            except:
                pass

    if not values:
        return {"error": "No numeric values found for key: " + key}

    total = sum(values)
    count = len(values)

    return {
        "key": key,
        "sum": total,
        "avg": total / count,
        "min": min(values),
        "max": max(values),
        "count": count
    }

def filter_data(data, field, operator, value):
    """
    Filter a list of dicts based on a condition.

    Args:
        data: List of dicts to filter
        field: Field name to check
        operator: Comparison operator (eq, ne, gt, lt, gte, lte, contains)
        value: Value to compare against

    Returns:
        Filtered list
    """
    result = []

    for item in data:
        if field not in item:
            continue

        item_value = item[field]

        match = False
        if operator == "eq":
            match = item_value == value
        elif operator == "ne":
            match = item_value != value
        elif operator == "gt":
            match = float(item_value) > float(value)
        elif operator == "lt":
            match = float(item_value) < float(value)
        elif operator == "gte":
            match = float(item_value) >= float(value)
        elif operator == "lte":
            match = float(item_value) <= float(value)
        elif operator == "contains":
            match = str(value) in str(item_value)

        if match:
            result.append(item)

    return {"data": result, "count": len(result)}

def group_by(data, key):
    """
    Group data by a field value.

    Args:
        data: List of dicts
        key: Field to group by

    Returns:
        Dict with groups
    """
    groups = {}

    for item in data:
        if key not in item:
            continue

        group_key = str(item[key])
        if group_key not in groups:
            groups[group_key] = []
        groups[group_key].append(item)

    return {
        "groups": groups,
        "group_count": len(groups)
    }

# Example usage
sample_data = [
    {"name": "Alice", "department": "Engineering", "salary": 100000},
    {"name": "Bob", "department": "Engineering", "salary": 95000},
    {"name": "Charlie", "department": "Sales", "salary": 85000},
    {"name": "Diana", "department": "Sales", "salary": 90000}
]

result = group_by(sample_data, "department")
```

**Usage:**

```
Tool: script_run
Args: {
  "name": "transform",
  "function": "aggregate_numbers",
  "args": [[{"value": "10"}, {"value": "20"}, {"value": "30"}], "value"]
}
```

---

## Currency Conversion Script

Convert between currencies using exchange rate API.

```python
# @description: Currency conversion using exchange rates
# @author: OpenPact Team
# @version: 1.0.0
# @secrets: EXCHANGE_RATE_API_KEY

def get_rate(from_currency, to_currency):
    """
    Get the current exchange rate between two currencies.

    Args:
        from_currency: Source currency code (e.g., "USD")
        to_currency: Target currency code (e.g., "EUR")

    Returns:
        Dict with exchange rate
    """
    api_key = secrets.get("EXCHANGE_RATE_API_KEY")
    if not api_key:
        return {"error": "EXCHANGE_RATE_API_KEY not configured"}

    from_currency = from_currency.upper().strip()
    to_currency = to_currency.upper().strip()

    url = format(
        "https://v6.exchangerate-api.com/v6/%s/pair/%s/%s",
        api_key,
        from_currency,
        to_currency
    )

    resp = http.get(url)
    if resp["status"] != 200:
        return {"error": "Exchange rate API failed", "status": resp["status"]}

    data = json.decode(resp["body"])

    if data["result"] != "success":
        return {"error": "Currency conversion failed", "details": data}

    return {
        "from": from_currency,
        "to": to_currency,
        "rate": data["conversion_rate"],
        "updated_at": data["time_last_update_utc"]
    }

def convert(amount, from_currency, to_currency):
    """
    Convert an amount from one currency to another.

    Args:
        amount: Amount to convert
        from_currency: Source currency code
        to_currency: Target currency code

    Returns:
        Dict with conversion result
    """
    rate_result = get_rate(from_currency, to_currency)

    if "error" in rate_result:
        return rate_result

    converted = float(amount) * rate_result["rate"]

    return {
        "original_amount": amount,
        "from_currency": from_currency,
        "to_currency": to_currency,
        "rate": rate_result["rate"],
        "converted_amount": round(converted, 2),
        "formatted": format("%s %.2f = %s %.2f",
            from_currency, amount,
            to_currency, converted
        )
    }

# Default execution
result = convert(100, "USD", "EUR")
```

**Usage:**

```
Tool: script_run
Args: {"name": "currency", "function": "convert", "args": [100, "USD", "GBP"]}
```

---

## API Health Check Script

Monitor the health of external APIs.

```python
# @description: Check health status of external APIs
# @author: OpenPact Team
# @version: 1.0.0
# @secrets: none

def check_endpoint(url, expected_status=200, timeout_threshold_ms=5000):
    """
    Check if an API endpoint is responding correctly.

    Args:
        url: URL to check
        expected_status: Expected HTTP status code
        timeout_threshold_ms: Response time threshold in milliseconds

    Returns:
        Dict with health status
    """
    start_time = time.now()

    try:
        resp = http.get(url)
        end_time = time.now()

        healthy = resp["status"] == expected_status

        return {
            "url": url,
            "healthy": healthy,
            "status_code": resp["status"],
            "expected_status": expected_status,
            "checked_at": end_time
        }
    except:
        return {
            "url": url,
            "healthy": False,
            "error": "Request failed",
            "checked_at": time.now()
        }

def check_multiple(endpoints):
    """
    Check health of multiple endpoints.

    Args:
        endpoints: List of dicts with 'name' and 'url' keys

    Returns:
        Dict with overall status and individual results
    """
    results = []
    all_healthy = True

    for endpoint in endpoints:
        name = endpoint.get("name", endpoint["url"])
        url = endpoint["url"]
        expected = endpoint.get("expected_status", 200)

        result = check_endpoint(url, expected)
        result["name"] = name

        results.append(result)

        if not result["healthy"]:
            all_healthy = False

        # Small delay between checks
        if len(results) < len(endpoints):
            time.sleep(0.5)

    healthy_count = len([r for r in results if r["healthy"]])

    return {
        "all_healthy": all_healthy,
        "healthy_count": healthy_count,
        "total_count": len(results),
        "results": results,
        "checked_at": time.now()
    }

# Example: Check common public APIs
endpoints = [
    {"name": "GitHub API", "url": "https://api.github.com"},
    {"name": "JSONPlaceholder", "url": "https://jsonplaceholder.typicode.com/posts/1"},
    {"name": "HTTPBin", "url": "https://httpbin.org/get"}
]

result = check_multiple(endpoints)
```

**Usage:**

```
Tool: script_run
Args: {
  "name": "health",
  "function": "check_multiple",
  "args": [[
    {"name": "My API", "url": "https://api.myservice.com/health"},
    {"name": "Database", "url": "https://db.myservice.com/status"}
  ]]
}
```

---

## Tips for Writing Scripts

### 1. Always Handle Errors

```python
def safe_api_call(url):
    resp = http.get(url)
    if resp["status"] != 200:
        return {"error": "API failed", "status": resp["status"]}
    return json.decode(resp["body"])
```

### 2. Validate Inputs

```python
def process(value):
    if not value:
        return {"error": "Value is required"}
    if type(value) != "string":
        return {"error": "Value must be a string"}
    # Process...
```

### 3. Use Descriptive Metadata

```python
# @description: Clear description of what this does
# @author: Your name
# @version: 1.0.0
# @secrets: LIST, OF, SECRETS
```

### 4. Return Structured Data

```python
# Good
return {"temperature": 20, "unit": "celsius", "city": "London"}

# Not ideal
return "The temperature in London is 20 celsius"
```

### 5. Include Timestamps

```python
return {
    "data": result,
    "fetched_at": time.now()
}
```
