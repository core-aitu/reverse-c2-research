#!/usr/bin/env python3
import argparse
import hashlib
import json
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        body = json.dumps({"models": [{"name": "fixture-a"}, {"name": "fixture-b"}]}).encode()
        event = {
            "timestampUtc": datetime.now(timezone.utc).isoformat(),
            "method": "GET",
            "path": self.path,
            "headerNames": sorted(self.headers.keys()),
            "responseSha256": hashlib.sha256(body).hexdigest(),
        }
        with open(self.server.log_path, "a", encoding="utf-8") as stream:
            stream.write(json.dumps(event, sort_keys=True) + "\n")
        if self.path != "/api/tags":
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        pass


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--log", required=True)
    args = parser.parse_args()
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.log_path = args.log
    server.serve_forever()
