import hashlib
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('release', Path(__file__).with_name('publish.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseGateTests(unittest.TestCase):
    def execute(self, failure=None, existing=False):
        evidence = json.loads(release.EVIDENCE_PATH.read_text())
        source = evidence['sourceCommit']
        mutations = []
        self.payloads = []
        body = release.NOTES_PATH.read_text()
        archive = Path('docs/security/evidence') / evidence['evidenceArchive']['path']
        assets = {'docker-compose.yml': b'fixture', 'ican.env.example': b'fixture',
                  'check-container.py': b'fixture',
                  'publication.json': release.EVIDENCE_PATH.read_bytes(),
                  'image-security-evidence.zip': archive.read_bytes()}
        assets['SHA256SUMS'] = ''.join(hashlib.sha256(data).hexdigest() + '  ' + name + '\n'
                                      for name, data in assets.items()).encode()
        draft = {'id': 42, 'tag_name': release.TAG, 'draft': not existing, 'prerelease': True, 'target_commitish': source, 'body': 'old notes' if existing else body, 'html_url': f'https://github.com/ruoyeyu/ICAN/releases/tag/{release.TAG}'}
        def api(path, method='GET', data=None, binary=False):
            if method != 'GET':
                mutations.append((path, method))
                if not binary:
                    self.payloads.append(data)
                if binary:
                    return {'digest': 'invalid' if failure == 'upload' else 'sha256:' + hashlib.sha256(data).hexdigest()}
                if method == 'PATCH':
                    return {**draft, **data, 'draft': False}
                return draft
            if path.endswith('/jobs?per_page=100'):
                return {'jobs': [{'name': n, 'conclusion': 'success'} for n in ('isolated-build', 'publish')]}
            if '/actions/runs/' in path:
                return {'head_sha': 'wrong' if failure == 'source' else source,
                        'conclusion': 'failure' if failure == 'checks' else 'success',
                        'status': 'completed', 'head_branch': 'main',
                        'path': '.github/workflows/ci.yml', 'event': 'push'}
            if '/git/ref/tags/' in path:
                return {'object': {'sha': source}}
            if path == '/releases?per_page=100':
                return [draft] if existing else []
            if '/assets?' in path:
                return [{'name': name, 'digest': 'bad' if failure == 'assets' else 'sha256:' + hashlib.sha256(data).hexdigest()}
                        for name, data in assets.items()] if existing else []
            raise AssertionError(path)
        inspected = [{'Id': 'wrong' if failure == 'image' else evidence['imageId'],
                      'Config': {'Labels': {'org.opencontainers.image.revision': source, 'org.opencontainers.image.version': 'wrong' if failure == 'version' else release.TAG.removeprefix('v')}}}]
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {
            'GITHUB_REPOSITORY': release.REPO, 'GITHUB_REF': 'refs/heads/main',
            'GITHUB_STEP_SUMMARY': str(Path(directory) / 'summary'),
        }), patch.object(release, 'request', side_effect=api), patch.object(release, 'optional', return_value=None), \
                patch.object(release, 'source_file', side_effect=lambda p, s: release.TAG.encode() if p == 'VERSION' else b'fixture'), \
                patch.object(release.subprocess, 'run'), \
                patch.object(release.subprocess, 'check_output', return_value=json.dumps(inspected).encode()):
            if failure:
                with self.assertRaises(RuntimeError):
                    release.main()
            else:
                release.main()
        return mutations

    def test_checks_and_artifact_mismatch_block_mutation(self):
        for failure in ('checks', 'source', 'image', 'version'):
            with self.subTest(failure=failure):
                self.assertEqual(self.execute(failure), [])

    def test_upload_failure_leaves_unpublished_draft(self):
        mutations = self.execute('upload')
        self.assertFalse(any(method == 'PATCH' for _, method in mutations))

    def test_existing_release_changes_only_notes(self):
        self.assertEqual(self.execute(existing=True), [('/releases/42', 'PATCH')])
        self.assertEqual(set(self.payloads[0]), {'body'})

    def test_existing_asset_mismatch_blocks_notes_update(self):
        self.assertEqual(self.execute('assets', existing=True), [])

    def test_publish_only_after_all_six_assets(self):
        mutations = self.execute()
        self.assertEqual(sum('/assets?' in path for path, _ in mutations), 6)
        self.assertEqual(mutations[-1], ('/releases/42', 'PATCH'))


if __name__ == '__main__':
    unittest.main()
