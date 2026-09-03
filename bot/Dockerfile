FROM python:3.14-slim

WORKDIR /app

ARG PIPER_MODELS="de_DE-thorsten-medium"

RUN apt-get update && apt-get install -y \
    ffmpeg \
    wget \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir piper-tts

# Download selected Piper voices.
RUN set -eux; \
    mkdir -p /app/piper-voices; \
    for model in ${PIPER_MODELS}; do \
        case "$model" in \
            de_DE-thorsten-medium) \
                base="https://huggingface.co/rhasspy/piper-voices/resolve/main/de/de_DE/thorsten/medium/de_DE-thorsten-medium"; \
                ;; \
            de_DE-thorsten_emotional-medium) \
                base="https://huggingface.co/rhasspy/piper-voices/resolve/main/de/de_DE/thorsten_emotional/medium/de_DE-thorsten_emotional-medium"; \
                ;; \
            en_US-lessac-medium) \
                base="https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium"; \
                ;; \
            *) \
                echo "Unknown Piper model: $model"; \
                exit 1; \
                ;; \
        esac; \
        wget -q -O "/app/piper-voices/${model}.onnx" "${base}.onnx"; \
        wget -q -O "/app/piper-voices/${model}.onnx.json" "${base}.onnx.json"; \
    done

COPY requirements.txt .

RUN pip install --no-cache-dir -r requirements.txt

COPY ./src/ .

CMD ["python", "main.py"]