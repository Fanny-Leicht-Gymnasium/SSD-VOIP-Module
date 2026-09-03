import os
import inspect
import sys
from types import SimpleNamespace

# Default values
DEFAULT_ARI_URL = "http://127.0.0.1:8088"
DEFAULT_ARI_USER = "bot"
DEFAULT_ARI_PASSWORD = "secret"
DEFAULT_APP_NAME = "ivrbot"
DEFAULT_TARGET_NUMBER = "**621"
DEFAULT_CALL_ENDPOINT = "fritzbox-endpoint"

def _to_env_key(default_name: str) -> str:
    # DEFAULT_ARI_URL -> ARI_URL
    return default_name.replace("DEFAULT_", "")


def load_env_config(set_globals=True) -> SimpleNamespace:
    """Auto-load all DEFAULT_* variables and override with ENV."""

    current_module = sys.modules[__name__]

    config_data = {}

    for name, default_value in inspect.getmembers(current_module):
        if not name.startswith("DEFAULT_"):
            continue
        

        env_key = _to_env_key(name)
        config_key = env_key.lower()

        env_val = os.getenv(env_key)
        if env_val is not None:
            config_data[config_key] = env_val
            if set_globals:
                globals()[name] = env_val
        else:
            config_data[config_key] = default_value
        print(f"Loading {name}: {env_val if env_val is not None else "(default)"} (default: {default_value})")
    print("Loaded env finished")
    return SimpleNamespace(**config_data)

load_env_config()