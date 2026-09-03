import asyncio
from collections.abc import Callable
import os
import time
import json
import requests
import websocket
import threading
import logging
from urllib.parse import urlsplit

import util.tts as tts
import util.env as env

from .callSession import CallSession
    
class ARIIVR:
    def __init__(
        self,
        ari_url=env.DEFAULT_ARI_URL,
        ari_user=env.DEFAULT_ARI_USER,
        ari_password=env.DEFAULT_ARI_PASSWORD,
        call_endpoint=env.DEFAULT_CALL_ENDPOINT,
        app_name=env.DEFAULT_APP_NAME,
        target_number=env.DEFAULT_TARGET_NUMBER,
    ):
        self.log = logging.getLogger("ARIIVR")

        self.ari_url = ari_url
        self.ari_user = ari_user
        self.ari_password = ari_password
        self.call_endpoint = call_endpoint
        self.app_name = app_name

        self.target_number = target_number

        self.sessions: dict[str, CallSession] = {}
        
        self.lock = threading.Lock()
        # Event set when the ARI websocket is open and the app is registered
        self.ws_ready = threading.Event()
        self.external_dtmf_handler: Callable[[str, str], None] = None
        self.external_call_end_handler: Callable[[str], None] = None
        self.external_call_accept_handler: Callable[[str], None] = None


    def get_session(self, channel_id: str) -> CallSession:
        with self.lock:
            if channel_id not in self.sessions:
                self.sessions[channel_id] = CallSession(channel_id, asterisk_session=self)
            return self.sessions[channel_id]
    
    def has_session(self, channel_id: str) -> CallSession|None:
        with self.lock:
            return self.sessions.get(channel_id)
    
    # ----------------------------
    # CALL CONTROL
    # ----------------------------
    def start_call(self, number: str, channel_id:str = "originate") -> CallSession:
        if not number:
            self.log.error("No target number specified for call originate")
            return None
        session = self.get_session(channel_id)
        if session.connected:
            self.log.warning(f"Session {channel_id} already connected, cannot start new call")
            return session
        # Ensure the ARI websocket is connected and the app is registered
        if not self.ws_ready.is_set():
            self.log.info("Waiting for ARI websocket registration before originate")
            # wait up to 5 seconds for registration
            if not self.ws_ready.wait(timeout=5):
                self.log.warning("ARI websocket not ready after timeout — aborting originate")
                return session

        url = f"{self.ari_url}/ari/channels/{channel_id}"

        params = {
            "endpoint": f"PJSIP/{number}@{self.call_endpoint}",
            "extension": "s",
            "context": "default",
            "priority": 1,
            "app": self.app_name,
        }

        r = requests.post(url, params=params, auth=(self.ari_user, self.ari_password))
        self.log.info("CALL ORIGINATE to %s: %s", number, r.status_code)
        self.log.debug("CALL: %s %s", r.status_code, r.text)
        
        return session

    def hangup(self, channel_id: str):
        url = f"{self.ari_url}/ari/channels/{channel_id}"
        r = requests.delete(url, auth=(self.ari_user, self.ari_password))
        self.log.info("HANGUP: %s", r.status_code)

    def set_external_dtmf_handler(self, handler: Callable[[str, str], None]):
        self.external_dtmf_handler = handler
    
    def set_external_call_end_handler(self, handler: Callable[[str], None]):
        self.external_call_end_handler = handler
        
    def set_external_call_accept_handler(self, handler: Callable[[str], None]):
        self.external_call_accept_handler = handler
        
        
    # ----------------------------
    # PLAYBACK QUEUE
    # ----------------------------

    def _play_sound(self, channel_id: str, sound: str):
        if sound is None:
            return
        
        url = f"{self.ari_url}/ari/channels/{channel_id}/play"

        r = requests.post(
            url,
            params={"media": f"sound:{sound}"},
            auth=(self.ari_user, self.ari_password),
        )

        self.log.info("PLAY START: %s (%s)", sound, r.status_code)

    def playback_finished(self, channel_id: str):
        session = self.get_session(channel_id)

        with self.lock:
            session.on_playback_finished()


    # ----------------------------
    # DTMF HANDLER
    # ----------------------------

    def _handle_dtmf(self, channel_id: str, digit: str):
        if self.external_dtmf_handler:
            self.external_dtmf_handler(channel_id, digit)
            return

        self.log.info("DTMF [%s]: %s", channel_id, digit)

    # ----------------------------
    # asterisk WEBSOCKET
    # ----------------------------
    def ws_listener(self):
        ari = urlsplit(self.ari_url.rstrip("/"))
        ws_scheme = "wss" if ari.scheme == "https" else "ws"
        ws_url = (
            f"{ws_scheme}://{ari.netloc}/ari/events"
            f"?api_key={self.ari_user}:{self.ari_password}"
            f"&app={self.app_name}"
        )

        def on_open(ws):
            self.log.info("WS CONNECTED")
            # mark websocket as ready so originates can proceed
            try:
                self.ws_ready.set()
            except Exception:
                pass

        def on_message(ws, message):
            event = json.loads(message)
            etype = event.get("type")
            self.log.info("WS EVENT: %s", message)
            if etype == "StasisStart":
                self.log.info("Startevent")
                channel_id = event["channel"]["id"]
                session = self.get_session(channel_id)
                with self.lock:
                    self.log.info("Startevent in lock")
                    session.connected = True
                    session.play_queue()
                self.log.info("CALL START: %s", channel_id)
                if self.external_call_accept_handler:
                    self.external_call_accept_handler(channel_id)

            elif etype == "ChannelDtmfReceived":
                channel_id = event["channel"]["id"]
                digit = event["digit"]
                self._handle_dtmf(channel_id, digit)

            elif etype == "PlaybackFinished":
                channel_id = event.get("playback", {}).get("target_uri", "").split(":")[-1]
                self.playback_finished(channel_id)

            elif etype == "ChannelDestroyed":
                channel_id = event["channel"]["id"]
                session = self.get_session(channel_id)
                with self.lock:
                    session.connected = False
                    session.call_ended = True
                    self.sessions.pop(channel_id, None)
                if self.external_call_end_handler:
                    self.external_call_end_handler(channel_id)
                self.log.info("CALL END: %s", channel_id)

        def on_error(ws, error):
            self.log.error("WS ERROR: %r", error)

        def on_close(ws, *_):
            self.log.warning("WS CLOSED")
            try:
                self.ws_ready.clear()
            except Exception:
                pass

        while True:
            self.ws_ready.clear()
            self.log.info("Connecting to ARI websocket: %s", ws_url)
            ws = websocket.WebSocketApp(
                ws_url,
                on_open=on_open,
                on_message=on_message,
                on_error=on_error,
                on_close=on_close,
            )

            try:
                # ARI is local; do not route its websocket handshake through a proxy.
                ws.run_forever(
                    ping_interval=20,
                    ping_timeout=10,
                    http_proxy_host=None,
                    http_proxy_port=None,
                )
            except Exception:
                self.log.exception("ARI websocket listener failed")
            finally:
                self.ws_ready.clear()

            self.log.info("Retrying ARI websocket connection in 2 seconds")
            time.sleep(2)

    # ----------------------------
    # RUN
    # ----------------------------

    def run(self):
        threading.Thread(target=self.ws_listener, daemon=True).start()
        self.log.info("IVR RUNNING")


if __name__ == "__main__":
    bot = ARIIVR()
    bot.run()
    
    session2 = bot.get_session("test2")
    asyncio.run(session2.play_tts("Auto TTS estellt."))
    session2.add_multiple_to_queue(["test/test","test/test","test/test"])
    session2.call("**612")
    
    
    session1 = bot.get_session("originate")
    session1.add_multiple_to_queue(["general/welcome"])
    bot.start_call(bot.target_number, "originate")
    
    
    