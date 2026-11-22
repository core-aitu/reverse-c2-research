#!/usr/bin/env python3
import argparse
import hashlib
import json
import socketserver
from datetime import datetime, timezone
from pathlib import Path


def utc_now():
    return datetime.now(timezone.utc).isoformat()


class Handler(socketserver.StreamRequestHandler):
    def record(self, direction, line):
        with self.server.transcript.open("a", encoding="utf-8") as stream:
            stream.write(json.dumps({"timestampUtc": utc_now(), "direction": direction, "line": line}) + "\n")

    def send(self, line):
        self.record("server", line)
        self.wfile.write((line + "\r\n").encode("ascii"))
        self.wfile.flush()

    def handle(self):
        self.send("220 localhost Specter runtime SMTP fixture")
        while True:
            raw = self.rfile.readline()
            if not raw:
                break
            line = raw.decode("utf-8", "replace").rstrip("\r\n")
            self.record("client", line)
            command = line.split(" ", 1)[0].upper()
            if command in {"EHLO", "HELO"}:
                self.send("250-localhost")
                self.send("250 SIZE 1048576")
            elif command in {"MAIL", "RCPT", "RSET", "NOOP"}:
                self.send("250 OK")
            elif command == "DATA":
                self.send("354 End data with <CR><LF>.<CR><LF>")
                chunks = []
                while True:
                    data_line = self.rfile.readline()
                    if not data_line or data_line == b".\r\n":
                        break
                    chunks.append(data_line)
                message = b"".join(chunks)
                self.server.message.write_bytes(message)
                self.record("message", json.dumps({
                    "size": len(message),
                    "sha256": hashlib.sha256(message).hexdigest(),
                }, sort_keys=True))
                self.send("250 2.0.0 queued")
            elif command == "QUIT":
                self.send("221 Bye")
                break
            else:
                self.send("250 OK")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--listen", type=int, required=True)
    parser.add_argument("--transcript", required=True)
    parser.add_argument("--message", required=True)
    args = parser.parse_args()
    transcript = Path(args.transcript)
    transcript.parent.mkdir(parents=True, exist_ok=True)
    transcript.write_text("", encoding="utf-8")
    with socketserver.ThreadingTCPServer(("127.0.0.1", args.listen), Handler) as server:
        server.transcript = transcript
        server.message = Path(args.message)
        print(json.dumps({"listen": args.listen, "transcript": args.transcript, "message": args.message}), flush=True)
        server.serve_forever()


if __name__ == "__main__":
    main()
