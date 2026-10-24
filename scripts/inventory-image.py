"""Inventory native code in the built image without running its binaries.

Read docker export as a tar stream; never extract archive paths or follow links.
The runner's readelf parses a temporary regular file, not application code.
"""
import hashlib
import json
import pathlib
import subprocess
import tarfile
import tempfile

image = "ican:verify"
container = subprocess.check_output(["docker", "create", image], text=True).strip()
result = {"image": subprocess.check_output(["docker", "image", "inspect", image, "--format", "{{.Id}}"], text=True).strip(),
          "nativeFiles": [], "setIdFiles": [], "licenses": []}
try:
    export = subprocess.Popen(["docker", "export", container], stdout=subprocess.PIPE)
    with tarfile.open(fileobj=export.stdout, mode="r|") as archive:
        for entry in archive:
            if not entry.isfile():
                continue
            if entry.mode & 0o6000:
                result["setIdFiles"].append(entry.name)
            name = entry.name.lower()
            if "license" in name or "copyright" in name:
                result["licenses"].append(entry.name)
            with archive.extractfile(entry) as source:
                magic = source.read(4)
                kind = ("ELF" if magic == b"\x7fELF" else "WASM" if magic == b"\0asm" else
                        "PE" if magic[:2] == b"MZ" else None)
                if not kind:
                    continue
                digest = hashlib.sha256(magic)
                with tempfile.NamedTemporaryFile(prefix="ican-native-") as binary:
                    binary.write(magic)
                    while chunk := source.read(1024 * 1024):
                        digest.update(chunk)
                        binary.write(chunk)
                    binary.flush()
                    item = {"path": entry.name, "kind": kind, "size": entry.size, "sha256": digest.hexdigest()}
                    if kind == "ELF":
                        dynamic = subprocess.run(["readelf", "-d", binary.name], capture_output=True, text=True, check=True)
                        symbols = subprocess.run(["readelf", "--dyn-syms", "--wide", binary.name], capture_output=True, text=True, check=True)
                        item["needed"] = [line.strip() for line in dynamic.stdout.splitlines() if "(NEEDED)" in line]
                        item["undefinedSymbols"] = [line.split(" UND ", 1)[1].strip() for line in symbols.stdout.splitlines() if " UND " in line]
                    result["nativeFiles"].append(item)
    if export.wait() != 0:
        raise RuntimeError("docker export failed")
finally:
    subprocess.run(["docker", "rm", container], check=True, stdout=subprocess.DEVNULL)
pathlib.Path("image-native-inventory.json").write_text(json.dumps(result, indent=2) + "\n")
print(json.dumps({"nativeFiles": [{k: v for k, v in f.items() if k in ["path", "kind", "sha256"]} for f in result["nativeFiles"]], "setIdFiles": result["setIdFiles"]}, indent=2))
assert not result["setIdFiles"], "Unexpected setuid/setgid file"
assert not any(f["path"].startswith("app/web/") for f in result["nativeFiles"]), "Unexpected native code in the web runtime"
