#!/usr/bin/env python3
"""Sync only changed, non-secret source files to the Node2Mint Ubuntu host."""

from __future__ import annotations

import argparse
import hashlib
import io
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tarfile

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_TARGET = os.getenv("N2M_TARGET", "node2mint-lan")
DEFAULT_DEST = os.getenv("N2M_DEST", "/home/n2m-deploy/node2mint")
MANIFEST = ".source-manifest"
EXCLUDED_PARTS = {".git", ".next", "node_modules", "__pycache__", ".secrets", "backups"}
# Only runtime artifacts are shipped. Audit/QA material (qa/, evidence/, reports/,
# tasks/, scripts/, *.md at root) is repo history, not deploy input — server never
# reads it, so syncing it just copies megabytes of screenshots to production.
SHIPPED_TOPS = {"gateway", "web", "deploy", "docker-compose.yml", "docker-compose.override.yml"}
EXCLUDED_FILES = {"deploy/gateway.env", "gateway/.media-private-probe.png"}


def run(cmd: list[str], *, input: bytes | None = None, check: bool = True) -> subprocess.CompletedProcess[bytes]:
    return subprocess.run(cmd, input=input, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=check)


def eligible(path: str) -> bool:
    pure = Path(path)
    if path in EXCLUDED_FILES or any(part in EXCLUDED_PARTS for part in pure.parts):
        return False
    return pure.parts[0] in SHIPPED_TOPS


def local_manifest() -> dict[str, str]:
    listed = run(["git", "-C", str(ROOT), "ls-files", "-co", "--exclude-standard", "-z"]).stdout.split(b"\0")
    files = sorted(item.decode() for item in listed if item and eligible(item.decode()))
    manifest: dict[str, str] = {}
    for rel in files:
        source = ROOT / rel
        if source.is_file():
            manifest[rel] = hashlib.sha256(source.read_bytes()).hexdigest()
    return manifest


def remote_manifest(target: str, dest: str) -> dict[str, str]:
    result = run(["ssh", "-o", "BatchMode=yes", target, f"cat {shlex.quote(dest)}/{MANIFEST}"], check=False)
    if result.returncode:
        return {}
    rows = (line.split("  ", 1) for line in result.stdout.decode().splitlines() if "  " in line)
    return {path: digest for digest, path in rows}


def write_remote(target: str, command: str, payload: bytes) -> None:
    result = run(["ssh", "-o", "BatchMode=yes", target, command], input=payload, check=False)
    if result.returncode:
        raise RuntimeError(result.stderr.decode().strip() or "remote write failed")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--initial", action="store_true", help="first migration: write every source file")
    parser.add_argument("--deploy", action="store_true", help="rebuild and restart production compose after sync")
    parser.add_argument("--prune", action="store_true", help="remove files on the server that are no longer shipped")
    parser.add_argument("--yes", action="store_true", help="with --prune: actually delete instead of listing")
    parser.add_argument("--target", default=DEFAULT_TARGET)
    parser.add_argument("--dest", default=DEFAULT_DEST)
    args = parser.parse_args()

    if args.prune:
        # One-time catch-up: the old manifest never pruned, so the server still
        # holds whatever was once tracked. Remove exactly what the manifest
        # claims, never a tree walk.
        remote = remote_manifest(args.target, args.dest)
        stale = sorted(path for path in remote if not eligible(path))
        if not stale:
            print("prune: nothing superfluous on the server")
            return 0
        print(f"prune: {len(stale)} files to remove (e.g. {', '.join(stale[:5])})")
        if not args.yes:
            print("re-run with --yes to actually delete")
            return 0
        script = (
            f"cd {shlex.quote(args.dest)} && while IFS= read -r file; do rm -f -- \"$file\"; done && "
            "find . -mindepth 1 -depth -type d -empty -delete"
        )
        write_remote(args.target, script, ("\n".join(stale) + "\n").encode())
        kept = "".join(f"{digest}  {path}\n" for path, digest in sorted(local_manifest().items()))
        write_remote(args.target, f"cat > {shlex.quote(args.dest)}/{MANIFEST}", kept.encode())
        print(f"prune: {len(stale)} deleted, manifest rewritten")
        return 0

    local = local_manifest()
    remote = {} if args.initial else remote_manifest(args.target, args.dest)
    changed = sorted(path for path, digest in local.items() if remote.get(path) != digest)
    deleted = sorted(path for path in remote if path not in local)
    if not changed and not deleted:
        print("sync: no source changes")
        return 0

    setup = f"mkdir -p {shlex.quote(args.dest)}"
    run(["ssh", "-o", "BatchMode=yes", args.target, setup])

    if changed:
        command = f"tar -xf - -C {shlex.quote(args.dest)}"
        process = subprocess.Popen(["ssh", "-o", "BatchMode=yes", args.target, command], stdin=subprocess.PIPE, stderr=subprocess.PIPE)
        assert process.stdin is not None and process.stderr is not None
        with tarfile.open(fileobj=process.stdin, mode="w|") as archive:
            for rel in changed:
                archive.add(ROOT / rel, arcname=rel, recursive=False)
        process.stdin.close()
        stderr = process.stderr.read().decode()
        if process.wait() != 0:
            raise RuntimeError(stderr.strip() or "source upload failed")

    if deleted:
        delete_script = f"cd {shlex.quote(args.dest)} && while IFS= read -r file; do rm -f -- \"$file\"; done"
        write_remote(args.target, delete_script, ("\n".join(deleted) + "\n").encode())

    serialized = "".join(f"{digest}  {path}\n" for path, digest in sorted(local.items()))
    write_remote(args.target, f"cat > {shlex.quote(args.dest)}/{MANIFEST}", serialized.encode())
    print(f"sync: {len(changed)} changed, {len(deleted)} deleted")

    if args.deploy:
        command = f"cd {shlex.quote(args.dest)}/deploy && docker compose -f docker-compose.prod.yml up -d --build"
        run(["ssh", "-o", "BatchMode=yes", args.target, command])
        print("deploy: compose updated")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, RuntimeError, subprocess.CalledProcessError) as error:
        print(f"sync failed: {error}", file=sys.stderr)
        raise SystemExit(1)
