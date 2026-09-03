import threading
from typing import TYPE_CHECKING
import util.tts as tts
import uuid
import os

import logging

logger = logging.getLogger(__name__)

if TYPE_CHECKING:
    from .asterisk import ARIIVR

class CallSession:
    def __init__(self, channel_id: str, asterisk_session:ARIIVR=None):
        self._lock = threading.Lock()
        if "/" in channel_id:
            raise RuntimeError("cannel_id can not contain /")
        self._channel_id: str = channel_id
        self._playback_queue: list[str] = []
        self._active_playback: str = None
        self._connected: bool = False
        self._call_ended: bool = False
        self._asterisk_session: ARIIVR = asterisk_session
        
        self._que_pause = False
    
    # --------------------
    # Thread-safe properties
    # --------------------
    @property
    def channel_id(self) -> str:
        return self._channel_id
    
    @property
    def call_ended(self) -> bool:
        with self._lock:
            return self._call_ended
        
    @call_ended.setter
    def call_ended(self, value: bool):
        with self._lock:
            if self._call_ended:
                raise RuntimeError("Cannot change call_ended once it's set to True")
            self._call_ended = bool(value)
        if bool(value):
            self.delete_all_tts()
    @property
    def playback_queue(self) -> list[str]:
        with self._lock:
            return list(self._playback_queue)

    @property
    def active_playback(self):
        with self._lock:
            return self._active_playback

    @active_playback.setter
    def active_playback(self, value):
        with self._lock:
            self._active_playback = value

    @property
    def connected(self) -> bool:
        with self._lock:
            return self._connected

    @connected.setter
    def connected(self, value: bool):
        with self._lock:
            self._connected = bool(value)
    
    def clear_queue(self):
        self.delete_all_tts()
        with self._lock:
            self._playback_queue = []
    def add_to_queue(self, sound: str):
        with self._lock:
            self._playback_queue.append(sound)
    def add_multiple_to_queue(self, sounds: list[str]):
        with self._lock:
            self._playback_queue.extend(sounds)
    def pop_from_queue(self, for_playing=False) -> str | None:
        with self._lock:
            if self._playback_queue:
                sound = self._playback_queue.pop(0)
                if sound is not None and sound.startswith("auto-tts/") and not for_playing:
                    self.delete_tts(sound)
                return sound
            else:
                return None
    
    def __repr__(self):
        return f"CallSession(channel_id={self.channel_id}, connected={self.connected}, active_playback={self.active_playback}, playback_queue={self.playback_queue})"
    def __str__(self):
        return self.__repr__()
    
    def on_playback_finished(self):
        if self.call_ended:
            raise RuntimeError("Session ended already.")

        with self._lock:
            if self._active_playback is not None and self._active_playback.startswith("auto-tts/"):
                self.delete_tts(self._active_playback)
            if self._que_pause:
                self._active_playback = None
            else:
                if len(self._playback_queue)>0:
                    self._active_playback = self._playback_queue.pop(0)
                    self._asterisk_session._play_sound(self.channel_id, self._active_playback)
                else:
                    self._active_playback = None
        
    def play_queue(self):
        if self.call_ended:
            raise RuntimeError("Session ended already.")
        if not self.connected:
            return
        with self._lock:
            self._que_pause = False
            if self._active_playback is None:
                if len(self._playback_queue)>0:
                    self._active_playback = self._playback_queue.pop(0)
                    #if self._active_playback is not None:
                    self._asterisk_session._play_sound(self.channel_id, self._active_playback)

            else:
                # TODO: Log already playing
                pass
    
    def pause_queue(self):
        """Will finish playing active file and then pause."""
        with self._lock:
            self._que_pause = True
    
    def call(self, number: str):
        if self.call_ended:
            raise RuntimeError("Reopening a Session is not allowed.")
        if self._asterisk_session:
            self._asterisk_session.start_call(number, self.channel_id)
    
    
    def hangup(self):
        if self.call_ended:
            raise RuntimeError("Session ended already.")
        if self._asterisk_session:
            self._asterisk_session.hangup(self.channel_id)
    
    
    def play(self, sound: str):
        self.add_to_queue(sound)
        self.play_queue()
    
    
    def play_multiple(self, sounds: list[str]):
        self.add_multiple_to_queue(sounds)
        self.play_queue()
    
    async def play_tts(self, text: str):
        logger.info("1111111111111111")
        if self.call_ended:
            raise RuntimeError("Session ended already.")
        logger.info("??????????")
        name = await self.create_tts(text)
        logger.info("created tts")
        if self.call_ended:
            self._delete_tts(name)
            raise RuntimeError("Session ended already.")
        self.add_to_queue(name)
        
        self.play_queue()
    
    def is_connected(self):
        return self.connected
    def is_playing(self):
        return self.active_playback is not None
    
    async def create_tts(self, text:str) -> None:
        name:str = f"auto-tts/{self.channel_id}-{uuid.uuid4()}"
        logger.info(f"creating {name}")

        await tts.generate_wav(text=text, output_name=name, output_dir="/app/sounds/")
        return name
    
    def delete_tts(self, name: str, dir: str = "/app/sounds/"):
        # Build file path
        path = os.path.join(dir, f"{name}.wav")

        # Delete file if it exists
        if os.path.isfile(path):
            os.remove(path)
    def delete_all_tts(self):
        for name in self.playback_queue:
            if name.startswith("auto-tts/"):
                self.delete_tts(name)
    