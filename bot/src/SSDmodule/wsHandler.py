import asyncio
import datetime
import json
import websockets

import logging

logger = logging.getLogger(__name__)

class WSHandler:
    def __init__(self, SystemType, SystemInfo=None, url="ws://localhost:8080/system/gsm", key="gsm-secret-key"):
        self.url = url
        self.key = key
        self.ws = None
        self.listen_task = None
        self.SystemType:str = SystemType
        self.SystemInfo:dict = SystemInfo if SystemInfo is not None else {
            "type": self.SystemType,
            "version": "1.0",
            "description": "SSD Module",
        }
        self.SystemErrors:str= """"""
        self.SystemHealth:str = "ok"
        # Reconnect settings
        self.reconnect_delay = 5
        self.running = False
        
    def set_health(self, health: str):
        self.SystemHealth = health
    def add_error(self, error: str):
        self.SystemErrors += error + "\n"
    def clear_errors(self):
        self.SystemErrors = ""
    def set_errors(self, errors: str):
        self.SystemErrors = errors
    async def connect(self):
        headers = {
            "Authorization": f"Bearer {self.key}",
        }

        url = f"{self.url}?client_type={self.SystemType}"

        self.ws = await websockets.connect(
            url,
            additional_headers=headers,
            ping_interval=20,
            ping_timeout=20,
        )

        logger.info("WebSocket connected")

        self.running = True

        # Start background receiver
        self.listen_task = asyncio.create_task(self._receive_loop())
    async def send_raw(self, message: str):
        if not self.ws:
            raise RuntimeError("WebSocket not connected")
        await self.ws.send(message)

    async def send(self, topic: str, type: str, thread_id: str, data: dict):
        timestamp = datetime.datetime.now().isoformat()
        logger.info(f"Preparing to send: type={type}, topic={topic}, thread_id={thread_id}, data={data}, timestamp={timestamp}")
        payload = {
            "type": type,
            "timestamp": timestamp, # 2026-06-03T12:24:35.256773772+02:00
        }
        if thread_id:
            payload["thread_id"] = thread_id
        if data:
            payload["data"] = data
        if topic:
            payload["topic"] = topic
        json_payload = json.dumps(payload)
        logger.info(f"Sending: {json_payload}")
        await self.send_raw(json_payload)

    async def _receive_loop(self):
        try:
            while True:
                msg = await self.ws.recv()
                await self.on_message(msg)
        except websockets.ConnectionClosed:
            logger.warning("Connection closed")

        except Exception:
            logger.exception("Receive loop failed")

        finally:
            self.ws = None

    async def on_message(self, message: str):
        try:
            data = json.loads(message)
            if isinstance(data.get("data"), str):
                try:
                    data["data"] = json.loads(data["data"])
                except json.JSONDecodeError:
                    logger.warning("Ignoring event with invalid JSON data: %s", data)
                    return
            await self.on_event(data)
        except json.JSONDecodeError:
            logger.info(f"Received non-JSON message: {message}")

    async def on_event(self, event: dict):
        logger.info(f"Event:{event}")
        match event.get("topic"):
            case "System":
                await self.on_system_event(event)
            case _:
                await self.on_custom_event(event)

    async def on_custom_event(self, event: dict):
        logger.info(f"Custom event: {event}")
        # handle custom events here
        
    async def on_system_event(self, event: dict):
        type = event.get("type")
        thread_id = event.get("thread_id")
        match type:
            case "connected":
                logger.info(f"System connected")
                await self.send("System", "type", thread_id, {type: self.SystemType, "info": self.SystemInfo})
                await self.send("System", "ping", thread_id, {})
            case "ping":
                logger.info(f"Ping received")
                await self.send("System", "pong", thread_id, {})
            case "status_request":
                logger.info(f"Status requested")
                await self.send("System", "status_response", thread_id, {"status": self.SystemHealth, "errors": self.SystemErrors, "info": self.SystemInfo})
            case "pong":
                logger.info(f"Pong received")
            case _:
                logger.info(f"Unknown system event type:{type}")
        
    async def close(self):
        self.running = False
        if self.listen_task:
            self.listen_task.cancel()

        if self.ws:
            await self.ws.close()
            self.ws = None
            logger.info(f"Disconnected")

    async def run(self):
        self.running = True
        await self._connection_loop()
        
    async def _connection_loop(self):
        while self.running:
            try:
                await self.connect()

                # Wait until receive loop exits
                await self.listen_task

            except Exception:
                logger.exception("Connection failed")

            if not self.running:
                break

            logger.info(
                "Disconnected from server. Reconnecting in %s seconds...",
                self.reconnect_delay,
            )

            await asyncio.sleep(self.reconnect_delay)


if __name__ == "__main__":
    WSHandler().run()