#!/usr/bin/env python3
import argparse
import hashlib
import http.client
import http.server
import json
import threading
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlsplit


HOP_BY_HOP = {
    "connection", "proxy-connection", "keep-alive", "transfer-encoding",
    "te", "trailer", "upgrade", "proxy-authenticate", "proxy-authorization",
}


def utc_now():
    return datetime.now(timezone.utc).isoformat()


class State:
    def __init__(self, output, target):
        self.output = Path(output)
        self.target = urlsplit(target) if target else None
        self.lock = threading.Lock()
        self.output.parent.mkdir(parents=True, exist_ok=True)
        self.output.write_text("", encoding="utf-8")

    def append(self, record):
        with self.lock, self.output.open("a", encoding="utf-8") as stream:
            stream.write(json.dumps(record, sort_keys=True) + "\n")


class Handler(http.server.BaseHTTPRequestHandler):
    server_version = "SpecterRuntimeCapture/1"

    def log_message(self, format, *args):
        return

    def _handle(self):
        length = int(self.headers.get("Content-Length", "0") or "0")
        body = self.rfile.read(length) if length else b""
        state = self.server.state
        record = {
            "timestampUtc": utc_now(),
            "method": self.command,
            "path": self.path,
            "headerNames": sorted({name.lower() for name in self.headers.keys()}),
            "bodySize": len(body),
            "bodySha256": hashlib.sha256(body).hexdigest(),
        }
        if not state.target:
            record["forwarded"] = False
            record["responseStatus"] = 502
            state.append(record)
            self.send_response(502)
            self.end_headers()
            return

        target_path = self.path
        if target_path.startswith("http://") or target_path.startswith("https://"):
            target_path = urlsplit(target_path).path or "/"
        connection = http.client.HTTPConnection(state.target.hostname, state.target.port, timeout=20)
        headers = {
            name: value for name, value in self.headers.items()
            if name.lower() not in HOP_BY_HOP and name.lower() != "host"
        }
        headers["Host"] = state.target.netloc
        try:
            connection.request(self.command, target_path, body=body, headers=headers)
            response = connection.getresponse()
            response_body = response.read()
            record["forwarded"] = True
            record["responseStatus"] = response.status
            record["responseSize"] = len(response_body)
            record["responseSha256"] = hashlib.sha256(response_body).hexdigest()
            state.append(record)
            self.send_response(response.status)
            for name, value in response.getheaders():
                if name.lower() not in HOP_BY_HOP and name.lower() != "content-length":
                    self.send_header(name, value)
            self.send_header("Content-Length", str(len(response_body)))
            self.end_headers()
            self.wfile.write(response_body)
        except Exception as exc:
            record["forwarded"] = True
            record["error"] = type(exc).__name__ + ": " + str(exc)
            record["responseStatus"] = 502
            state.append(record)
            self.send_response(502)
            self.end_headers()
        finally:
            connection.close()

    do_GET = _handle
    do_HEAD = _handle
    do_POST = _handle
    do_PUT = _handle
    do_DELETE = _handle
    do_PATCH = _handle

    def do_CONNECT(self):
        state = self.server.state
        record = {
            "timestampUtc": utc_now(),
            "method": "CONNECT",
            "path": self.path,
            "headerNames": sorted({name.lower() for name in self.headers.keys()}),
            "bodySize": 0,
            "bodySha256": hashlib.sha256(b"").hexdigest(),
            "forwarded": False,
            "responseStatus": 502,
        }
        state.append(record)
        self.send_response(502)
        self.end_headers()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--listen", type=int, required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--target")
    args = parser.parse_args()
    server = http.server.ThreadingHTTPServer(("127.0.0.1", args.listen), Handler)
    server.state = State(args.output, args.target)
    print(json.dumps({"listen": args.listen, "target": args.target, "output": args.output}), flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
