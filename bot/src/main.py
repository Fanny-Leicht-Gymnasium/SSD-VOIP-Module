import logging
import asterisk
import SSDmodule
from util import tts
import asyncio
import util.env as env

import logging

async def main():
    bot = asterisk.ARIIVR()
    bot.run()

    ssd_module = SSDmodule.CallModule(
        IVR=bot,
        url=env.DEFAULT_MODULE_WS_URL,
        key=env.DEFAULT_MODULE_WS_KEY,
    )
    await ssd_module.run()

    while True:
        await asyncio.sleep(60)

if __name__ == "__main__":
    logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s"
    )
    asyncio.run(main())