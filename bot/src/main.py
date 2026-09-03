import logging
import asterisk
import SSDmodule
from util import tts
import asyncio

import logging

async def main():
    await tts.generate_wav(
        "Willkommen beim IVR Bot. Drücke 1 für OK, 2 für Abbruch, 9 zum Beenden.",
        "general/welcome",
    )

    bot = asterisk.ARIIVR()
    bot.run()

    ssd_module = SSDmodule.CallModule(IVR=bot)
    await ssd_module.run()

    while True:
        await asyncio.sleep(60)

if __name__ == "__main__":
    logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s"
    )
    asyncio.run(main())