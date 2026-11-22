#!/usr/bin/env python3
import argparse
import hashlib
import json
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        size = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(size)
        record = {
            "timestampUtc": datetime.now(timezone.utc).isoformat(),
            "method": "POST",
            "path": self.path,
            "headerNames": sorted(name.lower() for name in self.headers.keys()),
            "hasAuthorization": "Authorization" in self.headers,
            "bodySize": len(body),
            "bodySha256": hashlib.sha256(body).hexdigest(),
            "responseStatus": 401,
        }
        with open(self.server.output, "a", encoding="utf-8") as stream:
            stream.write(json.dumps(record, sort_keys=True) + "\n")
        self.send_response(401)
        self.send_header("WWW-Authenticate", "NTLM")
        self.send_header("Content-Length", "0")
        self.end_headers()

    def log_message(self, *_):
        pass


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    open(args.output, "w", encoding="utf-8").close()
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.output = args.output
    server.serve_forever()
