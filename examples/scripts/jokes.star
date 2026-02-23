# jokes.star - Example Starlark script for OpenPact
# @description: Fetch random jokes from various APIs (no API key needed)
# @author: OpenPact
# @version: 1.0.0

def get_dad_joke():
    """
    Fetch a random dad joke from icanhazdadjoke.com

    Returns:
        Dict with the joke text
    """
    resp = http.get("https://icanhazdadjoke.com/", headers={
        "Accept": "application/json"
    })

    if resp["status"] != 200:
        return {"error": "Failed to fetch joke", "status": resp["status"]}

    data = json.decode(resp["body"])
    return {
        "joke": data["joke"],
        "id": data["id"]
    }

def get_chuck_norris():
    """
    Fetch a random Chuck Norris joke

    Returns:
        Dict with the joke text
    """
    resp = http.get("https://api.chucknorris.io/jokes/random")

    if resp["status"] != 200:
        return {"error": "Failed to fetch joke", "status": resp["status"]}

    data = json.decode(resp["body"])
    return {
        "joke": data["value"],
        "id": data["id"],
        "category": data.get("categories", ["uncategorized"])[0] if data.get("categories") else "uncategorized"
    }

def get_programming_joke():
    """
    Fetch a random programming joke

    Returns:
        Dict with the joke (setup + punchline or single)
    """
    resp = http.get("https://official-joke-api.appspot.com/jokes/programming/random")

    if resp["status"] != 200:
        return {"error": "Failed to fetch joke", "status": resp["status"]}

    data = json.decode(resp["body"])
    if len(data) == 0:
        return {"error": "No joke returned"}

    joke = data[0]
    return {
        "setup": joke["setup"],
        "punchline": joke["punchline"],
        "type": joke["type"]
    }

def random_joke():
    """
    Fetch a random joke from any source

    Returns:
        Dict with the joke
    """
    # Use current time to pseudo-randomly select a source
    now = time.now()
    # Simple hash based on last digit of timestamp
    choice = int(now[-2]) % 3

    if choice == 0:
        return get_dad_joke()
    elif choice == 1:
        return get_chuck_norris()
    else:
        return get_programming_joke()

# Default execution
result = random_joke()
