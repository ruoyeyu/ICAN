"""One-time, user-authorized version-label reset; remove after publication."""
import base64
import hashlib
import json
import os
import re
import subprocess
import publish

SOURCE = '53f8aa31d8fd803668cc138b3d91262ea9903c89'
EXPECTED = {'refs/tags/v0.1.0': '4702d09c590769f5ec73ef3de6b312702bcea7e3',
            'refs/tags/v0.1.3': '4cef8067138572047233b72e0fe67373bc1d3242'}
TAG = 'refs/tags/v0.1.0'
REMOTE = 'https://github.com/ruoyeyu/ICAN.git'


def validate_targets(refs, releases):
    actual = {}
    for item in refs:
        if item['ref'] not in EXPECTED or item['object']['type'] != 'commit':
            raise RuntimeError('Unexpected version tag; refusing reset')
        name, sha = item['ref'], item['object']['sha']
        if sha != EXPECTED[name] and not (name == TAG and sha == SOURCE):
            raise RuntimeError('Version tag changed; refusing reset')
        actual[name] = sha
    for release in releases:
        if (release['tag_name'] != publish.TAG or release['target_commitish'] != SOURCE
                or not release['prerelease']):
            raise RuntimeError('Unexpected release; refusing reset')
    return actual


def reset_tags():
    if os.environ.get('GITHUB_REPOSITORY') != publish.REPO or os.environ.get('GITHUB_REF') != 'refs/heads/main':
        raise RuntimeError('Reset only allowed on repository main')
    evidence = json.loads(publish.EVIDENCE_PATH.read_text())
    if evidence['sourceCommit'] != SOURCE or publish.TAG != 'v0.1.0':
        raise RuntimeError('Reset source mismatch')
    run_id = evidence['workflowRun'].removeprefix(f'https://github.com/{publish.REPO}/actions/runs/')
    if not run_id.isdecimal() or not re.fullmatch(r'ghcr.io/ruoyeyu/ican@sha256:[0-9a-f]{64}', evidence['image']):
        raise RuntimeError('Invalid checked publication')
    run = publish.request('/actions/runs/' + run_id)
    if (run['head_sha'] != SOURCE or run['conclusion'] != 'success' or run['status'] != 'completed'
            or run['head_branch'] != 'main' or run['path'] != '.github/workflows/ci.yml'
            or run['event'] not in ('push', 'workflow_dispatch')):
        raise RuntimeError('Checked source workflow required before tag reset')
    jobs = publish.request('/actions/runs/' + run_id + '/jobs?per_page=100')['jobs']
    if not all(any(j['name'] == name and j['conclusion'] == 'success' for j in jobs) for name in ('isolated-build', 'publish')):
        raise RuntimeError('Required publication jobs failed')
    if publish.source_file('VERSION', SOURCE).decode().strip() != publish.TAG:
        raise RuntimeError('Source version mismatch')
    archive = publish.EVIDENCE_PATH.parent / evidence['evidenceArchive']['path']
    if hashlib.sha256(archive.read_bytes()).hexdigest() != evidence['evidenceArchive']['sha256']:
        raise RuntimeError('Evidence checksum mismatch')
    subprocess.run(['docker', 'pull', evidence['image']], check=True)
    image = json.loads(subprocess.check_output(['docker', 'image', 'inspect', evidence['image']]))[0]
    if (image['Id'] != evidence['imageId'] or image['Config']['Labels']['org.opencontainers.image.revision'] != SOURCE
            or image['Config']['Labels']['org.opencontainers.image.version'] != '0.1.0'):
        raise RuntimeError('Checked image mismatch')
    refs_path = '/git/matching-refs/tags/v0.1.'
    actual = validate_targets(publish.request(refs_path), publish.request('/releases?per_page=100'))
    if actual != {TAG: SOURCE}:
        env = os.environ.copy()
        for key in tuple(env):
            if key.startswith('GIT_TRACE') or key == 'GIT_CURL_VERBOSE':
                env.pop(key)
        header = 'AUTHORIZATION: basic ' + base64.b64encode(('x-access-token:' + os.environ['GH_TOKEN']).encode()).decode()
        print('::add-mask::' + header, flush=True)
        env.update({'GIT_CONFIG_COUNT': '1', 'GIT_CONFIG_KEY_0': 'http.https://github.com/.extraheader',
                    'GIT_CONFIG_VALUE_0': header, 'GIT_TERMINAL_PROMPT': '0'})
        subprocess.run(['git', 'fetch', '--no-tags', REMOTE, SOURCE], env=env, check=True)
        leases = ['--force-with-lease=' + name + ':' + actual.get(name, '') for name in EXPECTED]
        updates = [SOURCE + ':' + TAG] + [':' + name for name in EXPECTED if name != TAG and name in actual]
        subprocess.run(['git', 'push', '--atomic', *leases, REMOTE, *updates], env=env, check=True)
    final = validate_targets(publish.request(refs_path), publish.request('/releases?per_page=100'))
    if final != {TAG: SOURCE}:
        raise RuntimeError('Unexpected final tag set')
    print('Version tags verified; regular publication checks run next')


if __name__ == '__main__':
    reset_tags()
    publish.main()
