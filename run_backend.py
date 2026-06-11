"""PyInstaller entry point for the warden backend."""
import asyncio
from agent.server import main

asyncio.run(main())
