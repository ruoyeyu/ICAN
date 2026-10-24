"""Read-only deployment checks. Never print Docker Env, secrets or full inspect output.

This checks container configuration, not firewall effectiveness or host patch status.
Run on the Docker host as the operator; never mount its socket inside the app.
"""
import argparse
import json
import pathlib
import re
import subprocess


def violations(container, image, data_dir, host_port):
    errors = []

    def require(ok, label):
        if not ok:
            errors.append(label)

    config = container.get("Config") or {}
    host = container.get("HostConfig") or {}
    expected = image.get("Config") or {}
    require(container.get("Image") == image.get("Id"), "image ID differs from reviewed artifact")
    reference = config.get("Image", "")
    require(bool(re.fullmatch(r"(?:[^\s]+@)?sha256:[0-9a-f]{64}", reference)), "mutable image reference")
    require(image.get("Os") == "linux" and image.get("Architecture") == "amd64", "platform not validated")
    require(config.get("User") == "10001:10001", "unexpected container user")
    for key in ("Entrypoint", "Cmd", "WorkingDir"):
        require(config.get(key) == expected.get(key), "startup override: " + key)
    require(bool(host.get("ReadonlyRootfs")), "root filesystem is writable")
    require(not host.get("Privileged"), "privileged container")
    require({x.upper() for x in host.get("CapDrop") or []} == {"ALL"}, "capabilities not fully dropped")
    for key in ("CapAdd", "Devices", "DeviceRequests", "DeviceCgroupRules", "VolumesFrom", "GroupAdd"):
        require(not host.get(key), "unexpected host access: " + key)
    security = {s.replace(":", "=", 1) for s in host.get("SecurityOpt") or []}
    require(bool(security & {"no-new-privileges", "no-new-privileges=true"}), "no-new-privileges missing")
    require(not any("unconfined" in s.lower() for s in security), "security profile disabled")
    require(host.get("PidMode", "") in ("", "private"), "shared PID namespace")
    require(host.get("IpcMode", "") in ("", "private"), "shared IPC namespace")
    require(host.get("UTSMode", "") in ("", "private"), "shared UTS namespace")
    require(host.get("CgroupnsMode") == "private", "shared cgroup namespace")
    require(host.get("UsernsMode", "") != "host", "user namespace isolation disabled")
    require(host.get("Init") is True, "init process missing")
    require(not host.get("OomKillDisable"), "OOM protection disabled")
    for key in ("PidsLimit", "Memory", "NanoCpus"):
        require(isinstance(host.get(key), (int, float)) and host[key] > 0, "resource limit missing: " + key)
    require(host.get("NetworkMode") not in ("host", "bridge", "none", "default", ""), "dedicated network missing")
    networks = (container.get("NetworkSettings") or {}).get("Networks") or {}
    require(len(networks) == 1, "unexpected number of attached networks")
    bindings = host.get("PortBindings") or {}
    require(bindings == {"3000/tcp": [{"HostIp": "127.0.0.1", "HostPort": str(host_port)}]}, "unexpected port exposure")
    require(not host.get("PublishAllPorts"), "all ports published")
    require(not host.get("ExtraHosts"), "unexpected extra host mappings")
    mounts = container.get("Mounts") or []
    data_mounts = [m for m in mounts if m.get("Destination") == "/app/data"]
    require(len(data_mounts) == 1, "data mount missing or duplicated")
    if len(data_mounts) == 1:
        mount = data_mounts[0]
        require(mount.get("Type") == "bind" and mount.get("Source") == data_dir
                and mount.get("RW") is True and mount.get("Propagation") == "rprivate", "unexpected data mount")
    allowed_tmp = {"/tmp", "/app/web/.next/cache"}
    require(all(m.get("Destination") == "/app/data" or
                (m.get("Type") == "tmpfs" and m.get("Destination") in allowed_tmp)
                for m in mounts), "unexpected extra mount")
    tmpfs = host.get("Tmpfs") or {}
    require(set(tmpfs) == allowed_tmp, "temporary filesystem boundary changed")
    for options in tmpfs.values():
        opts = set(options.split(","))
        require({"rw", "nosuid", "nodev", "noexec", "uid=10001", "gid=10001"} <= opts,
                "unsafe temporary filesystem options")
        require(any(re.fullmatch(r"size=[1-9][0-9]*[kKmMgG]?", o) for o in opts), "temporary storage limit missing")
    env = dict(v.split("=", 1) for v in config.get("Env") or [] if "=" in v)
    blocked = {"NODE_OPTIONS", "NODE_EXTRA_CA_CERTS", "LD_PRELOAD", "LD_LIBRARY_PATH", "LOCALDOMAIN",
               "RES_OPTIONS", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"}
    require(not any(value and key.upper() in blocked for key, value in env.items()), "unreviewed runtime environment override")
    require(env.get("NODE_TLS_REJECT_UNAUTHORIZED") != "0", "TLS verification disabled")
    return errors


def inspect(kind, target):
    return json.loads(subprocess.check_output(["docker", kind, "inspect", target], stderr=subprocess.PIPE))[0]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("container")
    parser.add_argument("--image-id", required=True)
    parser.add_argument("--data-dir", required=True)
    parser.add_argument("--host-port", required=True, type=int)
    args = parser.parse_args()
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", args.image_id):
        parser.error("--image-id must be the independently reviewed image ID")
    data_dir = pathlib.Path(args.data_dir)
    if not data_dir.is_absolute() or not data_dir.is_dir() or data_dir.resolve() != data_dir or data_dir == pathlib.Path("/"):
        parser.error("--data-dir must be an existing absolute directory, without symlinks")
    if not 1 <= args.host_port <= 65535:
        parser.error("invalid host port")
    try:
        errors = violations(inspect("container", args.container), inspect("image", args.image_id),
                            str(data_dir), args.host_port)
    except (subprocess.SubprocessError, OSError, ValueError, IndexError):
        raise SystemExit("Unable to inspect container/image; no deployment approval issued")
    print(json.dumps({"configurationChecks": "failed" if errors else "passed", "violations": errors,
                      "productionApproved": False,
                      "remaining": ["effective seccomp/AppArmor", "host patches and network ACL", "product and data acceptance"]}, indent=2))
    return bool(errors)


if __name__ == "__main__":
    raise SystemExit(main())
