import asyncio
import logging
import os
import subprocess
import tempfile

logger = logging.getLogger(__name__)


def _build_voice_map() -> dict[str, str]:
    """
    Build a {language_code: model_filename} map from the PIPER_MODELS
    env var (baked in at image build time, also passed through at
    runtime). Supports a comma-separated list of voice names, e.g.
    "de_DE-thorsten-medium,en_US-lessac-medium" for multi-language setups.

    The language code is derived from the model name's prefix, following
    Piper's "<lang>_<REGION>-<speaker>-<quality>" naming convention.
    """

    models = os.environ.get("PIPER_MODELS", "de_DE-thorsten-medium")

    voice_map: dict[str, str] = {}

    for name in (m.strip() for m in models.split(",")):
        if not name:
            continue

        lang = name.split("_", 1)[0]
        voice_map[lang] = f"{name}.onnx"

    return voice_map


VOICE_MAP = _build_voice_map()


async def generate_wav(
    text: str,
    output_name: str,
    output_dir: str = "/app/sounds",
    lang: str = "de",
    voice_dir: str = "/app/piper-voices",
) -> str:
    """
    Generate an Asterisk-compatible WAV using Piper + ffmpeg.

    Piper generates natural speech locally.
    ffmpeg converts it to 8 kHz mono PCM WAV.

    Returns:
        Path to the final WAV file.
    """

    output_path = os.path.join(output_dir, f"{output_name}.wav")
    os.makedirs(output_dir, exist_ok=True)

    voice_file = VOICE_MAP.get(lang)

    if not voice_file:
        raise ValueError(
            f"Unsupported language: {lang} "
            f"(configured voices: {', '.join(VOICE_MAP) or 'none'})"
        )

    model_path = os.path.join(voice_dir, voice_file)
    config_path = f"{model_path}.json"

    if not os.path.isfile(model_path):
        raise FileNotFoundError(
            f"Piper voice model not found: {model_path}"
        )

    if not os.path.isfile(config_path):
        raise FileNotFoundError(
            f"Piper voice config not found: {config_path} "
            "(each Piper model needs a matching .onnx.json file)"
        )

    with tempfile.NamedTemporaryFile(
        suffix=".wav",
        delete=False,
    ) as tmp:
        temp_path = tmp.name

    try:
        logger.info("Generating TTS with Piper")

        # Piper reads text from stdin and writes a WAV file.
        piper_result = await asyncio.to_thread(
            subprocess.run,
            [
                "piper",
                "--model",
                model_path,
                "--output_file",
                temp_path,
            ],
            input=text,
            text=True,
            capture_output=True,
            check=True,
        )

        if piper_result.stderr:
            logger.debug("Piper stderr: %s", piper_result.stderr)

        logger.info("Converting TTS audio to Asterisk format")

        # Convert Piper output to 8 kHz mono PCM WAV.
        ffmpeg_result = await asyncio.to_thread(
            subprocess.run,
            [
                "ffmpeg",
                "-y",
                "-i",
                temp_path,
                "-ar",
                "8000",
                "-ac",
                "1",
                "-c:a",
                "pcm_s16le",
                output_path,
            ],
            capture_output=True,
            text=True,
            check=True,
        )

        if ffmpeg_result.stderr:
            logger.debug("ffmpeg stderr: %s", ffmpeg_result.stderr)

        return output_path

    except subprocess.CalledProcessError as exc:
        logger.error(
            "TTS command failed with exit code %s: %s",
            exc.returncode,
            exc.stderr,
        )
        raise

    finally:
        if os.path.exists(temp_path):
            os.remove(temp_path)