# weather.star - Example Starlark script for OpenPact
# @description: Get current weather for any city using WeatherAPI.com
# @author: OpenPact
# @version: 1.0.0
# @secrets: WEATHER_API_KEY

def get_weather(city):
    """
    Fetch current weather from WeatherAPI.com

    Args:
        city: City name (e.g., "London", "Tokyo", "New York")

    Returns:
        Dict with temperature, condition, humidity, wind speed
    """
    api_key = secrets.get("WEATHER_API_KEY")
    if api_key == None:
        return {"error": "WEATHER_API_KEY not configured"}

    url = format(
        "https://api.weatherapi.com/v1/current.json?key=%s&q=%s",
        api_key,
        city
    )

    resp = http.get(url)

    if resp["status"] != 200:
        return {
            "error": "API request failed",
            "status": resp["status"]
        }

    data = json.decode(resp["body"])

    return {
        "city": data["location"]["name"],
        "country": data["location"]["country"],
        "temp_c": data["current"]["temp_c"],
        "temp_f": data["current"]["temp_f"],
        "condition": data["current"]["condition"]["text"],
        "humidity": data["current"]["humidity"],
        "wind_kph": data["current"]["wind_kph"],
        "last_updated": data["current"]["last_updated"]
    }

def get_forecast(city, days=3):
    """
    Fetch weather forecast from WeatherAPI.com

    Args:
        city: City name
        days: Number of forecast days (1-3)

    Returns:
        Dict with forecast data
    """
    api_key = secrets.get("WEATHER_API_KEY")
    if api_key == None:
        return {"error": "WEATHER_API_KEY not configured"}

    url = format(
        "https://api.weatherapi.com/v1/forecast.json?key=%s&q=%s&days=%d",
        api_key,
        city,
        days
    )

    resp = http.get(url)

    if resp["status"] != 200:
        return {"error": "API request failed", "status": resp["status"]}

    data = json.decode(resp["body"])
    forecast = []

    for day in data["forecast"]["forecastday"]:
        forecast.append({
            "date": day["date"],
            "max_temp_c": day["day"]["maxtemp_c"],
            "min_temp_c": day["day"]["mintemp_c"],
            "condition": day["day"]["condition"]["text"],
            "chance_of_rain": day["day"]["daily_chance_of_rain"]
        })

    return {
        "city": data["location"]["name"],
        "country": data["location"]["country"],
        "forecast": forecast
    }

# Default execution (when script is run without specifying a function)
result = get_weather("London")
