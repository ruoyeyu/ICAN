import contextlib
import io
import json
import os
import unittest
from unittest.mock import patch
import restart_release as reset


def ref(name, sha):
    return {'ref': name, 'object': {'type': 'commit', 'sha': sha}}


class RestartGateTests(unittest.TestCase):
    def execute(self, failure=None, finished=False):
        evidence = json.loads(reset.publish.EVIDENCE_PATH.read_text())
        initial = [ref(name, sha) for name, sha in reset.EXPECTED.items()]
        final = [ref(reset.TAG, reset.SOURCE)]
        if finished:
            initial = final
        if failure == 'tag':
            initial[0]['object']['sha'] = 'f' * 40
        if failure == 'extra_tag':
            initial.append(ref('refs/tags/v0.1.9', 'f' * 40))
        calls = []
        refs_read = 0
        def api(path):
            nonlocal refs_read
            if path.startswith('/git/matching-refs/'):
                refs_read += 1
                return initial if refs_read == 1 else final
            if path == '/releases?per_page=100':
                return [{'tag_name':'v0.1.7','target_commitish':'f'*40,'prerelease':True}] if failure == 'release' else []
            if path.endswith('/jobs?per_page=100'):
                return {'jobs':[{'name':n,'conclusion':'failure' if failure == 'job' else 'success'} for n in ('isolated-build','publish')]}
            return {'head_sha': reset.SOURCE, 'conclusion':'failure' if failure == 'checks' else 'success',
                    'status':'completed','head_branch':'main','path':'.github/workflows/ci.yml','event':'push'}
        image = [{'Id': 'wrong' if failure == 'image' else evidence['imageId'], 'Config':{'Labels':{
            'org.opencontainers.image.revision':reset.SOURCE, 'org.opencontainers.image.version':'0.1.0'}}}]
        with patch.dict(os.environ, {'GITHUB_REPOSITORY':reset.publish.REPO,'GITHUB_REF':'refs/heads/main','GH_TOKEN':'test-only'}), \
             patch.object(reset.publish,'request',side_effect=api), \
             patch.object(reset.publish,'source_file',return_value=b'wrong' if failure == 'version' else b'v0.1.0'), \
             patch.object(reset.subprocess,'check_output',return_value=json.dumps(image).encode()), \
             patch.object(reset.subprocess,'run',side_effect=lambda args,**kwargs:calls.append(args)), contextlib.redirect_stdout(io.StringIO()):
            if failure:
                with self.assertRaises(RuntimeError):reset.reset_tags()
            else:reset.reset_tags()
        return calls

    def test_failed_checks_or_changed_targets_never_push(self):
        for failure in ('checks','job','version','image','tag','extra_tag','release'):
            with self.subTest(failure=failure):
                self.assertFalse(any(c[:2] == ['git','push'] for c in self.execute(failure)))

    def test_atomic_push_has_exact_leases_and_only_authorized_refs(self):
        commands = self.execute()
        push = next(c for c in commands if c[:2] == ['git','push'])
        self.assertIn('--atomic',push)
        for name,sha in reset.EXPECTED.items():self.assertIn('--force-with-lease='+name+':'+sha,push)
        self.assertEqual(push[-2:],[reset.SOURCE+':'+reset.TAG,':refs/tags/v0.1.3'])

    def test_completed_reset_is_idempotent(self):
        self.assertFalse(any(c[0] == 'git' for c in self.execute(finished=True)))
