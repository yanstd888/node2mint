#!/usr/bin/env python3
"""Upstream OpenAI-compatible palsu untuk verify-endpoints.sh.

  python gateway/bin/mock-openai.py [port]   # default 18081

- POST /v1/chat/completions tanpa "stream": JSON penuh.
- ... dengan "stream": SSE (3 chunk) lalu data: [DONE].
"""
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

REPLY = ["Halo", " dari", " mock"]


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(n) or b"{}")
        model = body.get("model", "mock")
        if body.get("stream"):
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            for part in REPLY:
                chunk = {"object": "chat.completion.chunk", "model": model,
                         "choices": [{"index": 0, "delta": {"content": part}}]}
                self.wfile.write(("data: " + json.dumps(chunk) + "\n\n").encode())
            usage = {"object": "chat.completion.chunk", "model": model, "choices": [],
                     "usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8}}
            self.wfile.write(("data: " + json.dumps(usage) + "\n\n").encode())
            self.wfile.write(b"data: [DONE]\n\n")
            return
        out = {
            "id": "chatcmpl-mock", "object": "chat.completion", "model": model,
            "choices": [{"index": 0, "message": {"role": "assistant", "content": "".join(REPLY)}}],
            "usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
        }
        blob = json.dumps(out).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(blob)))
        self.end_headers()
        self.wfile.write(blob)

    def log_message(self, *a):  # jangan berisik
        pass


if __name__ == "__main__":
    HTTPServer(("0.0.0.0", int(sys.argv[1]) if len(sys.argv) > 1 else 18081), Handler).serve_forever()
