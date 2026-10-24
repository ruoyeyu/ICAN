"""Exercise the publication gate without network or registry credentials."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('publish-image.sh').resolve()
IMAGE_ID = 'sha256:' + 'a' * 64
REFERENCE = 'ghcr.io/ruoyeyu/ican@sha256:' + 'b' * 64


class PublishTests(unittest.TestCase):
    def run_case(self, failure=''):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'verified-image').mkdir()
            (root / 'verified-image/image-id.txt').write_text(IMAGE_ID + '\n')
            docker = root / 'docker'
            docker.write_text('''#!/usr/bin/env python3
import os, sys
args = sys.argv[1:]
if args[0] == 'login':
    sys.stdin.read()
if args[0] == 'push' and os.environ['FAILURE'] == 'push':
    sys.exit(42)
if args[:2] == ['image', 'inspect']:
    if 'RepoDigests' in args[-1]:
        print('bad-digest' if os.environ['FAILURE'] == 'digest' else os.environ['REFERENCE'])
    else:
        print('sha256:wrong' if os.environ['FAILURE'] == 'image' else os.environ['EXPECTED_ID'])
''')
            docker.chmod(0o700)
            env = {**os.environ, 'PATH': str(root) + ':' + os.environ['PATH'],
                   'GHCR_TOKEN': 'disposable-test-token', 'GITHUB_SHA': 'c' * 40,
                   'GITHUB_REPOSITORY': 'ruoyeyu/ICAN', 'GITHUB_REF': 'refs/heads/main',
                   'GITHUB_ACTOR': 'fixture', 'GITHUB_RUN_ID': '123', 'GITHUB_RUN_ATTEMPT': '1',
                   'GITHUB_STEP_SUMMARY': str(root / 'summary'), 'FAILURE': failure,
                   'EXPECTED_ID': IMAGE_ID, 'REFERENCE': REFERENCE}
            result = subprocess.run(['bash', str(SCRIPT)], cwd=root, env=env, capture_output=True, text=True)
            manifest = root / 'publication.json'
            record = json.loads(manifest.read_text()) if manifest.exists() else None
            self.assertNotIn(env['GHCR_TOKEN'], result.stdout + result.stderr + json.dumps(record))
            return result.returncode, record

    def test_success_records_source_and_digest(self):
        code, record = self.run_case()
        self.assertEqual(code, 0)
        self.assertEqual(record['image'], REFERENCE)
        self.assertEqual(record['sourceCommit'], 'c' * 40)
        self.assertFalse(record['productionApproved'])

    def test_failure_never_reports_success(self):
        for failure in ('push', 'digest', 'image'):
            with self.subTest(failure=failure):
                code, record = self.run_case(failure)
                self.assertNotEqual(code, 0)
                self.assertIsNone(record)


if __name__ == '__main__':
    unittest.main()
