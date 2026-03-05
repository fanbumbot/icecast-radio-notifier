import os
import sys
from datetime import datetime, timezone, timedelta
from typing import Awaitable, Any
from collections.abc import Callable

import asyncio
import aiohttp
import aiosqlite
from aiogram import Bot, Dispatcher, types, BaseMiddleware
from aiogram.exceptions import TelegramForbiddenError, TelegramNetworkError
from aiogram.filters import Command
from aiogram.enums import ChatType, ChatMemberStatus
from aiogram.types import TelegramObject, Message

from dotenv import load_dotenv
load_dotenv()

import importlib
import src.messages_template as msg

try:
    local_msg = importlib.import_module("src.messages")
    for key in dir(local_msg):
        if not key.startswith("__"):
            setattr(msg, key, getattr(local_msg, key))
except ImportError:
    pass

CURRENT_VERSION = 0

STREAM_URL = os.environ.get("STREAM_URL")
API_TOKEN = os.environ.get("TOKEN_ID")

if STREAM_URL is None:
    print("Environment variable STREAM_URL does not exist")
    exit()

if API_TOKEN is None:
    print("Environment variable TOKEN_ID does not exist")
    exit()

seconds_delay = 60 # 1 minute

DB_PATH = 'data/database.db'
DB_JOURNAL_PATH = 'data/journal.db'

bot = Bot(token=API_TOKEN)
dp = Dispatcher()

async def init_db():
    async with aiosqlite.connect(DB_PATH) as db:
        await db.execute("CREATE TABLE IF NOT EXISTS users (user_id INTEGER PRIMARY KEY, is_active INTEGER DEFAULT 1, last_used_version INTEGER DEFAULT 0)")
        await db.execute("CREATE TABLE IF NOT EXISTS chats (chat_id INTEGER PRIMARY KEY, is_active INTEGER DEFAULT 1, last_used_version INTEGER DEFAULT 0)")
        await db.execute("CREATE TABLE IF NOT EXISTS radio_state (key TEXT PRIMARY KEY, value TEXT DEFAULT 'False')")
        await db.commit()

    async with aiosqlite.connect(DB_JOURNAL_PATH) as db:
        await db.execute( \
"""
CREATE TABLE IF NOT EXISTS journal (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME,
    event_type TEXT
)
""")
        await db.commit()

async def get_radio_status():
    async with aiosqlite.connect(DB_PATH) as db:
        async with db.execute("SELECT value FROM radio_state WHERE key = 'site_status'") as cursor:
            row = await cursor.fetchone()
            if row is None or row[0] is None:
                return False
            try:
                return row[0] == "True"
            except:
                return False

async def set_radio_status(status: bool):
    status_text = str(status)
    current_timestamp = datetime.now(timezone.utc)
    log_server_status = "ServerStart" if status else "ServerStop"

    async with aiosqlite.connect(DB_PATH) as db:
        await db.execute("INSERT OR REPLACE INTO radio_state (key, value) VALUES ('site_status', ?)", (status_text,))
        await db.commit()

    async with aiosqlite.connect(DB_JOURNAL_PATH) as db:
        await db.execute("INSERT OR REPLACE INTO journal (timestamp, event_type) VALUES (?, ?)", (current_timestamp, log_server_status))
        await db.commit()

@dp.message(Command("start"))
async def start_cmd(message: types.Message):    
    if message.chat.type == ChatType.PRIVATE:
        if message.from_user is None:
            return
        async with aiosqlite.connect(DB_PATH) as db:
            await db.execute("INSERT OR IGNORE INTO users (user_id) VALUES (?) ON CONFLICT(user_id) DO UPDATE SET is_active=1", (message.from_user.id,))
            await db.commit()
    elif message.chat.type in [ChatType.GROUP, ChatType.SUPERGROUP]:
        async with aiosqlite.connect(DB_PATH) as db:
            await db.execute("INSERT OR IGNORE INTO chats (chat_id) VALUES (?)", (message.chat.id,))
            await db.commit()
    await message.answer(msg.START_MSG)

@dp.message(Command("stop"))
async def cmd_stop(message: types.Message):
    if message.from_user is None:
        return
    
    if message.chat.type == ChatType.PRIVATE:
        async with aiosqlite.connect(DB_PATH) as db:
            await db.execute("UPDATE users SET is_active = 0 WHERE user_id = ?", (message.from_user.id,))
            await db.commit()
    elif message.chat.type in [ChatType.GROUP, ChatType.SUPERGROUP]:
        async with aiosqlite.connect(DB_PATH) as db:
            await db.execute("UPDATE chats SET is_active = 0 WHERE chat_id = ?", (message.chat.id,))
            await db.commit()
    await message.answer(msg.STOP_MSG)

@dp.message(Command("help"))
async def cmd_help(message: types.Message):
    await message.answer(msg.HELP_MSG)

@dp.message(Command("status"))
async def cmd_status(message: types.Message):
    is_online = await get_radio_status()
    if is_online:
        await message.answer(msg.RADIO_ON_INFO)
    else:
        await message.answer(msg.RADIO_OFF_INFO)

@dp.message(Command("notification_status"))
async def cmd_notification_status(message: types.Message):
    if message.from_user is None:
        return
    
    rows = None

    if message.chat.type == ChatType.PRIVATE:
        async with aiosqlite.connect(DB_PATH) as db:
            async with db.execute("SELECT user_id FROM users WHERE user_id = ? AND is_active = 1", (message.from_user.id,)) as cursor:
                rows = await cursor.fetchall()
    elif message.chat.type in [ChatType.GROUP, ChatType.SUPERGROUP]:
        async with aiosqlite.connect(DB_PATH) as db:
            async with db.execute("SELECT chat_id FROM chats WHERE chat_id = ? AND is_active = 1", (message.chat.id,)) as cursor:
                rows = await cursor.fetchall()
    
    if rows is not None and len(list(rows)) > 0:
        await message.answer(msg.IS_SUBSCRIBED)
    else:
        await message.answer(msg.IS_NOT_SUBSCRIBED)

def get_timestamp_from_db_to_print(text: str):
    moscow_tz = timezone(timedelta(hours=3))
    timestamp = datetime.fromisoformat(text)
    moscow_timestamp = timestamp.astimezone(moscow_tz)
    return moscow_timestamp

@dp.message(Command("radio_hist"))
async def cmd_radio_hist(message: types.Message):
    if message.from_user is None:
        return
    
    async with aiosqlite.connect(DB_JOURNAL_PATH) as db:
        async with db.execute("SELECT timestamp, event_type FROM journal") as cursor:
            rows = await cursor.fetchall()

    output_rows = list()

    data: list[tuple[datetime, datetime | None]] = list()
    buffer = list(rows)

    start_idx = None
    end_idx = None
    current_idx = 0
    while True:
        if current_idx == len(buffer):
            if start_idx is not None and end_idx is None:
                timestamp_start_str = buffer[start_idx][0]
                timestamp_start = get_timestamp_from_db_to_print(timestamp_start_str)

                pair = (timestamp_start, None)
                data.append(pair)
            break
        row = buffer[current_idx]
        
        is_start = row[1] == "ServerStart"

        if is_start:
            if start_idx is None:
                start_idx = current_idx
            else:
                continue
        else:
            if start_idx is not None and end_idx is None:
                end_idx = current_idx
            else:
                continue

        if start_idx is not None and end_idx is not None:
            timestamp_start_str = buffer[start_idx][0]
            timestamp_start = get_timestamp_from_db_to_print(timestamp_start_str)

            timestamp_end_str = buffer[end_idx][0]
            timestamp_end = get_timestamp_from_db_to_print(timestamp_end_str)

            pair = (timestamp_start, timestamp_end)
            data.append(pair)

            start_idx = None
            end_idx = None

        current_idx += 1


    for start, end in data:
        if end is not None:
            formatted_start = start.strftime("%d/%m/%Y %H:%M")
            formatted_end = end.strftime("%d/%m/%Y %H:%M")
            text = msg.BROADCAST_HIST % (formatted_start, formatted_end)
        else:
            formatted_start = start.strftime("%d/%m/%Y %H:%M")
            text = msg.BROADCAST_HIST_NOW % (formatted_start,)
        output_rows.append(text)
    
    if len(output_rows) == 0:
        await message.answer(msg.NO_BROADCASTS_HIST)
    else:
        output_text = msg.BROADCAST_HIST_INIT + "\n" + "\n".join(output_rows)
        await message.answer(output_text)

async def send_to_all(text: str):
    async with aiosqlite.connect(DB_PATH) as db:
        async with db.execute("SELECT user_id FROM users WHERE is_active = 1") as cursor:
            rows = await cursor.fetchall()

    for row in rows:
        user_id = row[0]
        try:
            await bot.send_message(user_id, text)
            await asyncio.sleep(0.05)
        except TelegramForbiddenError:
            async with aiosqlite.connect(DB_PATH) as db:
                await db.execute("DELETE FROM users WHERE user_id = ?", (user_id,))
                await db.commit()
        except Exception:
            pass

    async with aiosqlite.connect(DB_PATH) as db:
        async with db.execute("SELECT chat_id FROM chats WHERE is_active = 1") as cursor:
            rows = await cursor.fetchall()

    for row in rows:
        chat_id = row[0]
        try:
            await bot.send_message(chat_id, text)
            await asyncio.sleep(0.05)
        except TelegramForbiddenError:
            async with aiosqlite.connect(DB_PATH) as db:
                await db.execute("DELETE FROM chats WHERE chat_id = ?", (chat_id,))
                await db.commit()
        except Exception:
            pass


async def radio_on():
    is_online = await get_radio_status()
    if not is_online:
        await set_radio_status(True)
        await send_to_all(msg.RADIO_ON_NOTIFICATION)

async def radio_off():
    is_online = await get_radio_status()
    if is_online:
        await set_radio_status(False)
        await send_to_all(msg.RADIO_OFF_NOTIFICATION)

async def check_icecast():
    global seconds_delay
    if STREAM_URL is None:
        return
    
    async with aiohttp.ClientSession() as session:
        while True:
            try:
                async with session.get(STREAM_URL) as response:
                    if response.status == 200:
                        await radio_on()
                    else:
                        await radio_off()
            except:
                await radio_off()
            await asyncio.sleep(seconds_delay)

class GroupAdminMiddleware(BaseMiddleware):
    async def __call__(
        self,
        handler: Callable[[TelegramObject, dict[str, Any]], Awaitable[Any]],
        event: TelegramObject,
        data: dict[str, Any]
    ) -> Any:
        if event.bot is None:
            return await handler(event, data)
        if not isinstance(event, Message):
            return await handler(event, data)

        if event.chat.type in [ChatType.GROUP, ChatType.SUPERGROUP]:
            if event.from_user is None:
                return await handler(event, data)
            
            member = await event.bot.get_chat_member(
                chat_id=event.chat.id, 
                user_id=event.from_user.id
            )
            
            is_admin = member.status in [
                ChatMemberStatus.ADMINISTRATOR, 
                ChatMemberStatus.CREATOR
            ] or event.sender_chat
            
            data["is_chat_admin"] = is_admin
            if not is_admin:
                return

        return await handler(event, data)
    
dp.message.middleware(GroupAdminMiddleware())

async def startup():
    asyncio.create_task(check_icecast())
    print("Bot has been started")

async def main():
    if sys.platform == 'win32':
        # Solution TelegramNetworkError in Windows (?)
        asyncio.set_event_loop_policy(asyncio.WindowsSelectorEventLoopPolicy())

    await init_db()
    dp.startup.register(startup)

    delay = 5
    max_retries = 3
    for attempt in range(1, max_retries + 1):
        try:
            await dp.start_polling(bot)
            break 
        except (TelegramNetworkError, Exception) as e:
            print(f"Attempt {attempt} failed: {e}")
            if attempt < max_retries:
                await asyncio.sleep(delay * attempt)
        finally:
            await bot.session.close()


if __name__ == "__main__":
    asyncio.run(main())