#!/usr/bin/env python3
"""Upstream palsu yang MENCATAT header Authorization yang diterima.

Bukti bahwa gateway mengirim API token milik PROVIDER (yang admin set di
/admin/providers), bukan token user pembeli:

  python bin/mock-echo.py [port] [file-log]   # default 18082 / gateway/bin/echo-log.txt

Log satu baris per panggilan: "model=<m> auth=<header Authorization>".
"""
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

LOG = sys.argv[2] if len(sys.argv) > 2 else "gateway/bin/echo-log.txt"

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(n) or b"{}")
        auth = self.headers.get("Authorization", "(kosong)")
        with open(LOG, "a", encoding="utf-8") as f:
            f.write("model=%s auth=%s\n" % (body.get("model", "?"), auth))
        out = {"id": "chatcmpl-echo", "object": "chat.completion", "model": body.get("model", "?"),
               "echo_authorization": auth,
               "choices": [{"index": 0, "message": {"role": "assistant", "content": "ok"}}],
               "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}}
        blob = json.dumps(out).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(blob)))
        self.end_headers()
        self.wfile.write(blob)
    def log_message(self, *a):
        pass

HTTPServer(("127.0.0.1", int(sys.argv[1] if len(sys.argv) > 1 else 18082)), Handler).serve_forever()
