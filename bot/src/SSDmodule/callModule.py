

import asyncio
import json
from typing import Callable

from asterisk import asterisk as asteriskLib
from asterisk.callSession import CallSession
from .wsHandler import WSHandler

import logging

logger = logging.getLogger(__name__)

class CallModule(WSHandler):
    def __init__(self, IVR:asteriskLib.ARIIVR, maxCalls:int=0, url="ws://localhost:8080/system/moduleWS", key="gsm-secret-key"):
        SystemType = "Call"
        SystemInfo = {
            "type": SystemType,
            "version": "1.0",
            "description": "SSD Call Module",
        }
        super().__init__(SystemType, SystemInfo, url, key) 
        self.asterisk: asteriskLib.ARIIVR = IVR
        self.maxCalls = maxCalls
        
        self.asterisk.set_external_dtmf_handler(self.on_asterisk_dtmf_event)
        self.asterisk.set_external_call_end_handler(self.on_asterisk_call_end)
        self.asterisk.set_external_call_accept_handler(self.on_asterisk_call_accept)
        
        self.dtmf_programming:dict[str, dict[str, callable]] = {}
    
    
    async def on_custom_event(self, event: dict):
        match event.get("topic"):
            case "Call":
                await self.on_call_event(event)
    
    async def on_call_event(self, event: dict):
        logger.info(f"Call event: {event}")
        event_type = event.get("type")
        thread_id = event.get("thread_id", "")
        session: CallSession = self.asterisk.has_session(thread_id)
        data = event.get("data", {})
        match event.get("type"):
            case "start_call":
                if session and session.connected:
                    logger.info(f"Call already active for thread_id {thread_id}")
                    await self.send("Call", "error", thread_id, {"message": "Call already active", "errorCode": "call_already_active"})
                    return
                logger.info(f"Starting call for thread_id {thread_id}")
                number = data.get("number")
                if number is None:
                    logger.info("No number provided for start_call event")
                    await self.send("Call", "error", thread_id, {"message": "No number provided", "errorCode": "no_number_provided"})
                    return
                if self.maxCalls > 0 and len(self.asterisk.sessions) >= self.maxCalls:
                    logger.info("Max call limit reached")
                    await self.send("Call", "error", thread_id, {"message": "Max call limit reached", "errorCode": "max_calls_reached"})
                    return
                self.asterisk.start_call(number, thread_id)
                
            case "end_call":
                if session and session.connected:
                    session.hangup()
                else:
                    logger.info(f"No active call session for thread_id {thread_id}")
                    await self.send("Call", "error", thread_id, {"message": "No active call session", "errorCode": "no_active_call_session"})
                    
            case "play_wav":
                if not session:
                    session = self.asterisk.get_session(thread_id)
                sound = data.get("sound")
                if sound is None:
                    logger.info("No sound provided for play_wav event")
                    await self.send("Call", "error", thread_id, {"message": "No sound provided", "errorCode": "no_sound_provided"})
                    return
                session.play(sound)
                
            case "play_tts":
                if not session:
                    session = self.asterisk.get_session(thread_id)
                text = data.get("text")
                if text is None:
                    logger.info("No text provided for play_tts event")
                    await self.send("Call", "error", thread_id, {"message": "No text provided", "errorCode": "no_text_provided"})
                    return
                logger.info("run play_tts event")

                asyncio.create_task(session.play_tts(text))
                
            case "clear_queue":
                if session and session.connected:
                    session.clear_queue()
                else:
                    logger.info(f"No active call session for thread_id {thread_id}")
                    await self.send("Call", "error", thread_id, {"message": "No active call session", "errorCode": "no_active_call_session"})

            case "skip_playback":
                if session and session.connected:
                    if not session.skip_playback():
                        await self.send("Call", "error", thread_id, {"message": "No active playback", "errorCode": "no_active_playback"})
                else:
                    await self.send("Call", "error", thread_id, {"message": "No active call session", "errorCode": "no_active_call_session"})

            case "end_call_after_queue":
                if session and session.connected:
                    session.set_end_call_after_queue(data.get("enabled", True))
                    logger.info(f"Set end_call_after_queue to {data.get('enabled', True)} for thread_id {thread_id}")
                else:
                    await self.send("Call", "error", thread_id, {"message": "No active call session", "errorCode": "no_active_call_session"})
                    
            case "dtmf_programming":
                if not session:
                    session = self.asterisk.get_session(thread_id)
                if session:
                    digit = data.get("digit")
                    if digit is None:
                        logger.info("No digit provided for dtmf_programming event")
                        await self.send("Call", "error", thread_id, {"message": "No digit provided for dtmf_programming event", "errorCode": "no_digit_provided"})
                        return

                    command = data.get("command")
                    if command is None:
                        logger.info("No command provided for dtmf_programming event")
                        await self.send("Call", "error", thread_id, {"message": "No command provided for dtmf_programming event", "errorCode": "no_command_provided"})
                        return
                    callback, error = self.parse_callback(command, session, digit, thread_id, self.asterisk)
                    if error:
                        logger.info(f"Error parsing callback: {error}")
                        await self.send("Call", "error", thread_id, {"message": f"Error parsing callback: {error}"})
                        return

                    if not self.program_dtmf_callback(thread_id, digit, callback=callback):
                        logger.info("Failed to program DTMF callback")
                        await self.send("Call", "error", thread_id, {"message": "Failed to program DTMF callback"})
                else:
                    logger.info(f"No active call session for thread_id {thread_id}")
                    await self.send("Call", "error", thread_id, {"message": "No active call session"})
                    
            case _:
                logger.info(f"Unknown call event type: {event.get("type")}")
                
                
    def parse_callback(self, command: dict, session: CallSession, digit: str, thread_id: str, asterisk) -> tuple[Callable[[], None] | None, str|None]:
    # For security reasons, we only allow predefined callbacks
        # In a real implementation, you would have a registry of allowed callbacks
        # Here we just return a dummy function for demonstration
        type: str = command.get("type")
        match type:
            case "play_sound":
                sound = command.get("arg")
                if not sound:
                    return None, "No arg specified for play_sound command"
                return lambda: session.play(sound), None
            case "play_tts":
                text = command.get("arg")
                if not text:
                    return None, "No arg specified for play_tts command"
                return lambda: asyncio.run(session.play_tts(text)), None
            case "hangup":
                return lambda: session.hangup(), None
            case _:
                return None, "Unknown command"
        return None, "Unknown command"

        
    def program_dtmf_callback(self, thread_id: str, digit: str, callback: Callable[[], None])-> bool:
        if thread_id not in self.dtmf_programming:
            self.dtmf_programming[thread_id] = {}
        if not callable(callback):
            logger.info("Provided callback is not callable")
            return False
        self.dtmf_programming[thread_id][digit] = callback
        return True
        
    def on_asterisk_dtmf_event(self, channel_id:str, digit: str):
        logger.info(f"Received DTMF event from Asterisk: {channel_id} {digit}")
        programmed = False
        if channel_id in self.dtmf_programming:
            if digit in self.dtmf_programming[channel_id]:
                callback = self.dtmf_programming[channel_id][digit]
                if callback and callable(callback):
                    programmed = True
                    logger.info(f"Executing programmed callback for channel {channel_id} and digit {digit}")
                    try:
                        callback()
                    except Exception as e:
                        logger.info(f"Error wile dtmf running programming {e}")
                
        asyncio.run(self.send("Call", "dtmf_event", channel_id, {"digit": digit, "programmed": programmed}))
        
    def on_asterisk_call_end(self, channel_id:str):
        logger.info(f"Call ended for channel_id: {channel_id}")
        self.dtmf_programming.pop(channel_id, None)
        asyncio.run(self.send("Call", "call_ended", channel_id, {}))
        
    def on_asterisk_call_accept(self, channel_id:str):
        logger.info(f"Call accept for channel_id: {channel_id}")
        asyncio.run(self.send("Call", "call_accepted", channel_id, {}))