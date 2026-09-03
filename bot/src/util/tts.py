import logging
import os
import subprocess
import tempfile
import asyncio

logger = logging.getLogger(__name__)

async def generate_wav(
    text: str,
    output_name: str,
    output_dir: str = "/app/sounds"
) -> str:
    """
    Generate Asterisk-compatible WAV (8kHz, mono) using espeak-ng + ffmpeg.
    Returns final file path.
    """

    output_path = os.path.join(output_dir, f"{output_name}.wav")
    output_dir = os.path.dirname(output_path)
    os.makedirs(output_dir, exist_ok=True)

    with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tmp:
        temp_path = tmp.name
    logger.info("run tts")
    try:
        # run espeak-ng in thread
        await asyncio.to_thread(
            subprocess.run,
            ["espeak-ng", "-w", temp_path, text],
            check=True
        )

        # run ffmpeg in thread
        await asyncio.to_thread(
            subprocess.run,
            [
                "ffmpeg",
                "-y",
                "-i", temp_path,
                "-ar", "8000",
                "-ac", "1",
                output_path
            ],
            capture_output=True,
            text=True
        )

        return output_path

    finally:
        if os.path.exists(temp_path):
            os.remove(temp_path)