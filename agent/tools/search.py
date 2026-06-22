from __future__ import annotations

import asyncio
import time
import urllib.error
import urllib.parse
import urllib.request
from typing import Any

from agent.tools.base import Tool, is_ssrf_safe_url


class GoogleSearchTool(Tool):
    name = "google_search"
    description = "Search the web. Returns top-5 results. Does not open the user's browser."
    params = {"query": {"type": "string", "description": "Search query"}}

    async def execute(self, args: dict[str, Any]) -> str:
        query = args.get("query", "")
        try:
            from duckduckgo_search import DDGS

            def _search() -> list:
                last_exc: Exception | None = None
                for attempt in range(3):
                    try:
                        with DDGS() as ddgs:
                            results = list(ddgs.text(query, max_results=5))
                        if results:
                            return results
                    except Exception as e:
                        last_exc = e
                    if attempt < 2:
                        time.sleep(1.5)
                if last_exc:
                    raise last_exc
                return []

            results = await asyncio.to_thread(_search)
            if not results:
                return "no results"
            return "\n".join(f"• {r['title']}\n  {r['href']}\n  {r.get('body', '')[:200]}" for r in results)
        except ImportError:
            return "error: pip install duckduckgo-search"
        except Exception as e:
            return f"error: {e}"


class WebFetchTool(Tool):
    name = "webfetch"
    description = (
        "Fetch content from a URL. Returns plain text or markdown. "
        "Supports HTML, JSON, and plain text responses. "
        "Max 5MB, 30s timeout. For interactive pages use browser_read instead."
    )
    params = {
        "url": {"type": "string", "description": "The URL to fetch content from"},
        "format": {
            "type": "string",
            "description": "Return format: text, markdown, or html (default: markdown)",
        },
        "timeout": {"type": "integer", "description": "Timeout in seconds (max 120, default 30)"},
        "verify_ssl": {"type": "boolean", "description": "Verify TLS certificate (default: true)"},
    }

    async def execute(self, args: dict[str, Any]) -> str:
        url = args.get("url", "")
        fmt = args.get("format", "markdown")
        timeout = min(int(args.get("timeout", 30)), 120)
        verify = args.get("verify_ssl", True)

        if not url.startswith("http://") and not url.startswith("https://"):
            return "error: URL must start with http:// or https://"

        if not is_ssrf_safe_url(url):
            return "error: URL is blocked (SSRF or file scheme)"

        # encode non-ASCII chars in path/query so urllib doesn't raise on Cyrillic etc.
        _p = urllib.parse.urlparse(url)
        url = urllib.parse.urlunparse(
            _p._replace(
                path=urllib.parse.quote(_p.path, safe="/:@!$&'()*+,;=%"),
                query=urllib.parse.quote(_p.query, safe="=&+%#"),
            )
        )

        headers = {
            "User-Agent": (
                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
                "AppleWebKit/537.36 (KHTML, like Gecko) "
                "Chrome/143.0.0.0 Safari/537.36"
            ),
            "Accept-Language": "en-US,en;q=0.9",
        }
        accept_map = {
            "markdown": "text/markdown;q=1.0, text/html;q=0.8, text/plain;q=0.7, */*;q=0.1",
            "text": "text/plain;q=1.0, text/html;q=0.8, */*;q=0.1",
            "html": "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8",
        }
        headers["Accept"] = accept_map.get(fmt, accept_map["markdown"])

        try:
            import ssl

            req = urllib.request.Request(url, headers=headers)
            ctx = ssl.create_default_context()
            if not verify:
                ctx.check_hostname = False
                ctx.verify_mode = ssl.CERT_NONE
            try:
                resp_cm = await asyncio.to_thread(urllib.request.urlopen, req, context=ctx, timeout=timeout)
            except urllib.error.URLError as e:
                if isinstance(getattr(e, "reason", None), ssl.SSLCertVerificationError):
                    return "error: SSL certificate verification failed (set verify_ssl=false to bypass)"
                raise
            with resp_cm as resp:
                content_type = resp.headers.get("Content-Type", "text/plain")
                raw = resp.read()
            content = raw.decode("utf-8", errors="replace")[:10000]

            if fmt == "markdown" and ("html" in content_type or content_type == "text/html"):
                return self._html_to_markdown(content)[:10000]
            elif fmt == "text" and ("html" in content_type or content_type == "text/html"):
                return self._html_to_text(content)[:10000]
            return content[:10000]
        except urllib.error.HTTPError as e:
            return f"error: HTTP {e.code} {e.reason}"
        except TimeoutError:
            return f"error: timeout {timeout}s"
        except Exception as e:
            return f"error: {e}"

    @staticmethod
    def _html_to_markdown(html: str) -> str:
        try:
            import html2text

            h = html2text.HTML2Text()
            h.body_width = 0
            h.ignore_links = False
            h.ignore_images = False
            h.ignore_emphasis = False
            h.protect_links = True
            h.unicode_snob = True
            h.skip_internal_links = True
            return h.handle(html).strip()
        except ImportError:
            return WebFetchTool._html_to_text(html)

    @staticmethod
    def _html_to_text(html: str) -> str:
        import re

        text = re.sub(r"<script[^>]*>.*?</script>", "", html, flags=re.DOTALL | re.IGNORECASE)
        text = re.sub(r"<style[^>]*>.*?</style>", "", text, flags=re.DOTALL | re.IGNORECASE)
        text = re.sub(r"<[^>]+>", "", text)
        text = re.sub(r"\s+", " ", text).strip()
        return text
