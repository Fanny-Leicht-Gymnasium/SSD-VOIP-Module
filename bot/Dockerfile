FROM python:3.14-slim-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y \
    wget \
    && rm -rf /var/lib/apt/lists/*

ARG PIPER_MODELS="de_DE-thorsten-medium"

# Download selected Piper voices.
RUN set -eux; \
    mkdir -p /app/piper-voices; \
    for model in ${PIPER_MODELS}; do \
        case "$model" in \
            none) \
                echo "No Piper voices selected"; \
                ;; \
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
        if [ "$model" = "none" ]; then \
            continue; \
        fi; \
        wget -q -O "/app/piper-voices/${model}.onnx" "${base}.onnx"; \
        wget -q -O "/app/piper-voices/${model}.onnx.json" "${base}.onnx.json"; \
    done

FROM python:3.14-slim-bookworm


RUN apt-get update && apt-get install -y \
    ffmpeg \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .

RUN pip install --no-cache-dir \
    -r requirements.txt

    
COPY ./src/ .

RUN mkdir -p /app/piper-voices

COPY --from=builder /app/piper-voices/ /app/piper-voices/

CMD ["python", "main.py"]