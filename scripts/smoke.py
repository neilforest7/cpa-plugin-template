#!/usr/bin/env python3
"""End-to-end ABI smoke test: build the library, load it, register and normalize.

Runs without a CLIProxyAPI install, so a plugin can be checked before tagging a release.

    python3 scripts/smoke.py
"""

from __future__ import annotations

import base64
import ctypes
import json
import platform
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PLUGIN_ID = "sample-normalizer"
SMOKE_VERSION = "0.0.0-smoke"
EXT = {"Darwin": "dylib", "Linux": "so", "Windows": "dll"}[platform.system()]


class Buffer(ctypes.Structure):
    _fields_ = [("ptr", ctypes.c_void_p), ("len", ctypes.c_size_t)]


class HostAPI(ctypes.Structure):
    _fields_ = [
        ("abi_version", ctypes.c_uint32),
        ("host_ctx", ctypes.c_void_p),
        ("call", ctypes.c_void_p),
        ("free_buffer", ctypes.c_void_p),
    ]


CallFn = ctypes.CFUNCTYPE(
    ctypes.c_int, ctypes.c_char_p, ctypes.POINTER(ctypes.c_uint8), ctypes.c_size_t, ctypes.POINTER(Buffer)
)
FreeFn = ctypes.CFUNCTYPE(None, ctypes.c_void_p, ctypes.c_size_t)
ShutdownFn = ctypes.CFUNCTYPE(None)


class PluginAPI(ctypes.Structure):
    _fields_ = [("abi_version", ctypes.c_uint32), ("call", CallFn), ("free_buffer", FreeFn), ("shutdown", ShutdownFn)]


def build(destination: Path) -> Path:
    library = destination / f"{PLUGIN_ID}.{EXT}"
    subprocess.run(
        [
            "go", "build", "-buildmode=c-shared",
            "-ldflags", f"-X main.pluginVersion={SMOKE_VERSION}",
            "-o", str(library), ".",
        ],
        cwd=ROOT,
        check=True,
    )
    return library


def main() -> int:
    checks = 0
    with tempfile.TemporaryDirectory() as tmp:
        library = build(Path(tmp))
        plugin = ctypes.CDLL(str(library))
        api = PluginAPI()
        if plugin.cliproxy_plugin_init(ctypes.byref(HostAPI(abi_version=1)), ctypes.byref(api)) != 0:
            print("cliproxy_plugin_init failed", file=sys.stderr)
            return 1
        if api.abi_version != 1:
            print(f"unexpected abi_version {api.abi_version}", file=sys.stderr)
            return 1
        checks += 1

        def call(method: str, payload: dict | None = None) -> dict:
            raw_request = json.dumps(payload).encode() if payload is not None else b""
            request = (ctypes.c_uint8 * len(raw_request)).from_buffer_copy(raw_request) if raw_request else None
            response = Buffer()
            code = api.call(method.encode(), request, len(raw_request), ctypes.byref(response))
            body = ctypes.string_at(response.ptr, response.len)
            api.free_buffer(response.ptr, response.len)
            try:
                envelope = json.loads(body)
            except json.JSONDecodeError as err:
                print(f"{method} returned invalid JSON ({err}): {body!r}", file=sys.stderr)
                raise SystemExit(1) from err
            if code != 0 or not envelope.get("ok"):
                print(f"{method} failed: {body!r}", file=sys.stderr)
                raise SystemExit(1)
            return envelope["result"]

        registered = call("plugin.register", {"schema_version": 6})
        if not registered["capabilities"]["request_normalizer"]:
            print("request_normalizer capability not declared", file=sys.stderr)
            return 1
        if registered["metadata"]["Version"] != SMOKE_VERSION:
            print(f"metadata version {registered['metadata']['Version']} != {SMOKE_VERSION}", file=sys.stderr)
            return 1
        checks += 1

        def normalize(body: dict) -> dict:
            result = call(
                "request.normalize",
                {
                    "FromFormat": "openai",
                    "ToFormat": "codex",
                    "Model": "gpt-5.5",
                    "Stream": False,
                    "Body": base64.b64encode(json.dumps(body).encode()).decode(),
                },
            )
            try:
                return json.loads(base64.b64decode(result["Body"]))
            except (KeyError, json.JSONDecodeError) as err:
                print(f"normalized body unusable ({err}): {result!r}", file=sys.stderr)
                raise SystemExit(1) from err

        original = {"model": "gpt-5.5", "input": "hi"}
        if normalize(original) != original:
            print("empty configuration must leave the body untouched", file=sys.stderr)
            return 1
        checks += 1

        call("plugin.reconfigure", {"config_yaml": base64.b64encode(b"set_fields:\n  service_tier: priority\n").decode()})
        updated = normalize(original)
        if updated.get("service_tier") != "priority" or updated.get("input") != "hi":
            print(f"set_fields not applied: {updated}", file=sys.stderr)
            return 1
        checks += 1

        api.shutdown()
    print(f"smoke: ok ({checks} checks, {PLUGIN_ID}.{EXT})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
