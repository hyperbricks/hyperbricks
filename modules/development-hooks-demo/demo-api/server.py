"""Small foreground HTTP API, deliberately independent of HyperBricks readiness."""
import argparse
import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from socketserver import TCPServer
from urllib.parse import urlsplit


class LoopbackHTTPServer(ThreadingHTTPServer):
    def server_bind(self):
        # HTTPServer normally resolves the bound address with getfqdn(). A
        # loopback-only demo needs no reverse DNS, which can stall on CI hosts.
        TCPServer.server_bind(self)
        self.server_name = "127.0.0.1"
        self.server_port = self.server_address[1]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=4331)
    parser.add_argument("--site-port", type=int, default=4330)
    args = parser.parse_args()
    session_file = Path(__file__).resolve().parent / ".runtime" / "session.json"
    session = json.loads(session_file.read_text(encoding="utf-8"))

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            path = urlsplit(self.path).path
            status = 200
            if path == "/health":
                # This must not call HyperBricks: its listener starts later.
                payload = {"ready": True, "pid": os.getpid()}
            elif path == "/message":
                payload = {
                    **session,
                    "message": "Your managed API is ready.",
                    "pid": os.getpid(),
                    "site_url": f"http://127.0.0.1:{args.site_port}",
                }
            else:
                status, payload = 404, {"error": "Not found"}
            body = json.dumps(payload).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Cache-Control", "no-store")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    # Binding errors fail the foreground process; do not adopt an existing API.
    print(f"Demo API binding 127.0.0.1:{args.port}", flush=True)
    server = LoopbackHTTPServer(("127.0.0.1", args.port), Handler)
    print(f"Demo API listening on http://127.0.0.1:{args.port}", flush=True)
    try:
        server.serve_forever()
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
