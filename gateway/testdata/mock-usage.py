#!/usr/bin/env python3
"""Mock upstream OpenAI-compatible dengan usage TETAP, untuk menguji tagihan
token (input+cache+output) x multiplier tanpa provider 9router asli.

Usage yang dikembalikan: prompt_tokens=1000, completion_tokens=500,
prompt_tokens_details.cached_tokens=200 (cache ikut prompt, sesuai OpenAI).
Jadi total token yang ditagih = 1000 + 500 = 1500.
"""
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 18099
LOG = sys.argv[2] if len(sys.argv) > 2 else None


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(n) or b"{}")
        if LOG:
            with open(LOG, "a", encoding="utf-8") as f:
                f.write("model=%s auth=%s stream=%s\n" % (
                    body.get("model", "?"), self.headers.get("Authorization", "-"), body.get("stream")))
        out = {
            "id": "chatcmpl-mockusage",
            "object": "chat.completion",
            "model": body.get("model", "?"),
            "choices": [{"index": 0, "message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}],
            "usage": {
                "prompt_tokens": 1000,
                "completion_tokens": 500,
                "total_tokens": 1500,
                "prompt_tokens_details": {"cached_tokens": 200},
            },
        }
        blob = json.dumps(out).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(blob)))
        self.end_headers()
        self.wfile.write(blob)


if __name__ == "__main__":
    HTTPServer(("0.0.0.0", PORT), Handler).serve_forever()
