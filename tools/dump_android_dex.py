#!/usr/bin/env python3
"""Dump valid in-memory DEX files from an Android process with Frida."""

from __future__ import annotations

import argparse
import sys
import time
from pathlib import Path

import frida


AGENT = r"""
'use strict';

const seen = new Set();
let dumping = false;

function emitDex(address, size, source) {
  const id = address.toString().replace('0x', '');
  send({type: 'dex-start', id: id, address: address.toString(), size: size, source: source});
  const chunkSize = 1024 * 1024;
  for (let offset = 0; offset < size; offset += chunkSize) {
    const length = Math.min(chunkSize, size - offset);
    send({type: 'dex-chunk', id: id, offset: offset}, address.add(offset).readByteArray(length));
  }
  send({type: 'dex-end', id: id});
}

function scanRanges() {
  if (dumping) return;
  dumping = true;
  send({type: 'status', message: 'scan started'});

  const ranges = [];
  for (const protection of ['r--', 'rw-', 'r-x', 'rwx']) {
    for (const range of Process.enumerateRanges({protection: protection, coalesce: false})) {
      if (range.size >= 0x70 && range.size <= 512 * 1024 * 1024) ranges.push(range);
    }
  }

  for (const range of ranges) {
    let matches;
    try {
      matches = Memory.scanSync(range.base, range.size, '64 65 78 0a 30 33 ?? 00');
    } catch (_) {
      continue;
    }
    for (const match of matches) {
      const key = match.address.toString();
      if (seen.has(key)) continue;
      seen.add(key);
      try {
        const headerSize = match.address.add(0x24).readU32();
        const endianTag = match.address.add(0x28).readU32();
        const fileSize = match.address.add(0x20).readU32();
        const available = range.base.add(range.size).sub(match.address).toUInt32();
        if (headerSize !== 0x70 || endianTag !== 0x12345678) continue;
        if (fileSize < 0x70 || fileSize > available || fileSize > 256 * 1024 * 1024) continue;
        emitDex(match.address, fileSize, range.file ? range.file.path : '[anonymous]');
      } catch (_) {
      }
    }
  }
  send({type: 'done', count: seen.size});
}

function hookLoader(name) {
  const target = Module.findGlobalExportByName(name);
  if (target === null) return false;
  Interceptor.attach(target, {
    onEnter(args) {
      let path = '';
      try { path = args[0].readCString(); } catch (_) {}
      if (path.indexOf('libmmkv.so') !== -1) {
        send({type: 'status', message: name + ' intercepted: ' + path});
        scanRanges();
      }
    }
  });
  return true;
}

const hooks = ['android_dlopen_ext', 'dlopen'].filter(hookLoader);
send({type: 'ready', hooks: hooks});
"""


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--package", default="com.mcdonalds.gma.cn")
    parser.add_argument("--output", type=Path, default=Path("research/dex-dump"))
    parser.add_argument("--timeout", type=float, default=30.0)
    args = parser.parse_args()

    args.output.mkdir(parents=True, exist_ok=True)
    files: dict[str, object] = {}
    completed = False

    def on_message(message, data):
        nonlocal completed
        if message.get("type") == "error":
            print(message.get("stack", message), file=sys.stderr)
            return
        payload = message.get("payload", {})
        kind = payload.get("type")
        if kind == "dex-start":
            path = args.output / f"dex-{payload['id']}-{payload['size']}.dex"
            files[payload["id"]] = path.open("w+b")
            print(f"[dex] {payload['address']} {payload['size']} bytes {payload['source']}")
        elif kind == "dex-chunk":
            handle = files[payload["id"]]
            handle.seek(payload["offset"])
            handle.write(data)
        elif kind == "dex-end":
            handle = files.pop(payload["id"])
            handle.close()
        elif kind == "done":
            completed = True
            print(f"[done] scanned candidates: {payload['count']}")
        elif kind in {"status", "ready"}:
            print(f"[{kind}] {payload}")

    device = frida.get_usb_device(timeout=10)
    pid = device.spawn([args.package])
    session = device.attach(pid)
    script = session.create_script(AGENT)
    script.on("message", on_message)
    script.load()
    device.resume(pid)

    deadline = time.monotonic() + args.timeout
    while not completed and time.monotonic() < deadline:
        time.sleep(0.1)

    for handle in files.values():
        handle.close()
    try:
        session.detach()
    except frida.InvalidOperationError:
        pass
    return 0 if completed else 1


if __name__ == "__main__":
    raise SystemExit(main())
