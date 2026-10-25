import copy
import argparse
import importlib.util
import pathlib
import unittest

spec = importlib.util.spec_from_file_location("policy", pathlib.Path(__file__).with_name("check-container.py"))
policy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(policy)


class ContainerPolicyTests(unittest.TestCase):
    def setUp(self):
        self.image = {"Id": "sha256:" + "a" * 64, "Os": "linux", "Architecture": "amd64",
                      "Config": {"Cmd": ["node", "/app/docker-entrypoint.mjs"], "WorkingDir": "/app"}}
        self.container = {
            "Image": self.image["Id"],
            "Config": {**self.image["Config"], "Image": self.image["Id"], "User": "10001:10001",
                       "Env": ["ADMIN_PASSWORD=fixture-only-secret"]},
            "HostConfig": {"ReadonlyRootfs": True, "CapDrop": ["ALL"], "SecurityOpt": ["no-new-privileges=true"],
                           "Init": True, "PidsLimit": 256, "Memory": 1610612736, "MemorySwap": 1610612736, "NanoCpus": 1600000000,
                           "CgroupnsMode": "private", "IpcMode": "private", "NetworkMode": "fixture_ican",
                           "PortBindings": {"3000/tcp": [{"HostIp": "127.0.0.1", "HostPort": "3001"}]},
                           "Tmpfs": {p: "rw,nosuid,nodev,noexec,size=33554432,uid=10001,gid=10001"
                                     for p in ("/tmp", "/app/web/.next/cache")}},
            "Mounts": [{"Type": "bind", "Source": "/fixture/data", "Destination": "/app/data", "RW": True,
                        "Propagation": "rprivate"}],
            "NetworkSettings": {"Networks": {"fixture_ican": {}}}}

    def check(self, container):
        return policy.violations(container, self.image, "/fixture/data", 3001, 1600000000, 1610612736)

    def test_expected_configuration(self):
        self.assertEqual(self.check(self.container), [])

    def test_operator_selected_limits_allow_other_servers(self):
        self.container["HostConfig"].update({"NanoCpus": 4000000000, "Memory": 8589934592, "MemorySwap": 8589934592})
        self.assertEqual(policy.violations(self.container, self.image, "/fixture/data", 3001,
                                           policy.cpu_quantity("4"), policy.memory_quantity("8g")), [])
        self.assertTrue(self.check(self.container))

    def test_resource_units_and_invalid_values(self):
        self.assertEqual(policy.memory_quantity("1.5g"), policy.memory_quantity("1536m"))
        self.assertEqual(policy.cpu_quantity("1.6"), 1600000000)
        for parse, values in ((policy.cpu_quantity, ("0", "-1", "NaN", "Infinity", "1g", "0.0000000001")),
                              (policy.memory_quantity, ("0", "-1", "NaN", "Infinity", "2tb", "0.1b"))):
            for value in values:
                with self.subTest(value=value), self.assertRaises(argparse.ArgumentTypeError):
                    parse(value)

    def test_independent_host_exposures(self):
        cases = {"Privileged": True, "ReadonlyRootfs": False, "CapAdd": ["SYS_ADMIN"],
                 "SecurityOpt": ["no-new-privileges=true", "seccomp=unconfined"], "PidsLimit": -1,
                 "Memory": 2147483648, "MemorySwap": -1, "NanoCpus": 2000000000, "NetworkMode": "host", "PidMode": "host", "IpcMode": "host",
                 "CgroupnsMode": "host", "Init": False, "DeviceRequests": [{"Count": -1}],
                 "PortBindings": {"3000/tcp": [{"HostIp": "0.0.0.0", "HostPort": "3001"}]}}
        for key, value in cases.items():
            with self.subTest(key=key):
                container = copy.deepcopy(self.container)
                container["HostConfig"][key] = value
                self.assertTrue(self.check(container))

    def test_socket_mount(self):
        self.container["Mounts"].append({"Type": "bind", "Source": "/var/run/docker.sock", "Destination": "/var/run/docker.sock"})
        self.assertIn("unexpected extra mount", self.check(self.container))

    def test_mutable_image_and_command_override(self):
        for key, value in (("Image", "ican:latest"), ("Cmd", ["node", "-e", "fixture"]), ("User", "0")):
            container = copy.deepcopy(self.container)
            container["Config"][key] = value
            self.assertTrue(self.check(container))

    def test_environment_override_does_not_leak(self):
        self.container["Config"]["Env"].append("https_proxy=http://fixture-secret@proxy.invalid")
        errors = self.check(self.container)
        self.assertTrue(errors)
        self.assertNotIn("fixture-secret", str(errors))
        self.assertNotIn("fixture-only-secret", str(errors))


if __name__ == "__main__":
    unittest.main()
