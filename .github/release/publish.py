"""Publish only the existing checked image; never build or deploy a server."""
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import urllib.error
import urllib.parse
import urllib.request

REPO = 'ruoyeyu/ICAN'
TAG = 'v0.1.3'
EVIDENCE_PATH = Path('docs/security/evidence/ghcr-v0.1.3-publication.json')
NOTES_PATH = Path('.github/release/v0.1.3.md')
API = f'https://api.github.com/repos/{REPO}'


def request(path, method='GET', data=None, binary=False):
    # Authenticated destinations are fixed; no artifact-supplied URLs are followed.
    url = f'https://uploads.github.com/repos/{REPO}{path}' if binary else API + path
    body = data if binary else (json.dumps(data).encode() if data is not None else None)
    req = urllib.request.Request(url, data=body, method=method, headers={
        'Authorization': 'Bearer ' + os.environ['GH_TOKEN'],
        'Accept': 'application/vnd.github+json', 'X-GitHub-Api-Version': '2022-11-28',
        'Content-Type': 'application/octet-stream' if binary else 'application/json',
    })
    with urllib.request.urlopen(req, timeout=60) as response:
        return json.load(response)


def optional(path):
    try:
        return request(path)
    except urllib.error.HTTPError as error:
        if error.code != 404:
            raise
        return None


def source_file(path, source):
    item = request('/contents/' + path + '?ref=' + source)
    if item.get('encoding') != 'base64':
        raise RuntimeError('Unexpected source file encoding')
    return base64.b64decode(item['content'])


def main():
    if os.environ.get('GITHUB_REPOSITORY') != REPO or os.environ.get('GITHUB_REF') != 'refs/heads/main':
        raise RuntimeError('Release only allowed from repository main')
    evidence_path = EVIDENCE_PATH
    evidence = json.loads(evidence_path.read_text())
    source, image = evidence['sourceCommit'], evidence['image']
    if not re.fullmatch(r'[0-9a-f]{40}', source) or not re.fullmatch(r'ghcr.io/ruoyeyu/ican@sha256:[0-9a-f]{64}', image):
        raise RuntimeError('Invalid immutable source or image reference')
    run_id = evidence['workflowRun'].removeprefix(f'https://github.com/{REPO}/actions/runs/')
    if not run_id.isdecimal():
        raise RuntimeError('Invalid check run')
    run = request('/actions/runs/' + run_id)
    if (run['head_sha'] != source or run['conclusion'] != 'success' or run['status'] != 'completed'
            or run['head_branch'] != 'main' or run['path'] != '.github/workflows/ci.yml'
            or run['event'] not in ('push', 'workflow_dispatch')):
        raise RuntimeError('Source has not passed the required publication workflow')
    jobs = request('/actions/runs/' + run_id + '/jobs?per_page=100')['jobs']
    if not all(any(j['name'] == name and j['conclusion'] == 'success' for j in jobs)
               for name in ('isolated-build', 'publish')):
        raise RuntimeError('Required build/publication jobs did not pass')
    if source_file('VERSION', source).decode().strip() != TAG:
        raise RuntimeError('Source VERSION differs from release tag')
    tag_ref = optional('/git/ref/tags/' + TAG)
    if tag_ref and (tag_ref['object']['type'] != 'commit' or tag_ref['object']['sha'] != source):
        raise RuntimeError('Existing tag differs; refusing to move it')
    subprocess.run(['docker', 'pull', image], check=True)
    inspected = json.loads(subprocess.check_output(['docker', 'image', 'inspect', image]))[0]
    if (inspected['Id'] != evidence['imageId']
            or inspected['Config']['Labels']['org.opencontainers.image.revision'] != source
            or inspected['Config']['Labels']['org.opencontainers.image.version'] != TAG.removeprefix('v')):
        raise RuntimeError('Registry image does not match the checked artifact')

    assets = {name: source_file(path, source) for name, path in {
        'docker-compose.yml': 'docker-compose.yml',
        'ican.env.example': '.env.example',
        'check-container.py': 'scripts/check-container.py',
    }.items()}
    archive = Path('docs/security/evidence') / evidence['evidenceArchive']['path']
    raw = archive.read_bytes()
    if hashlib.sha256(raw).hexdigest() != evidence['evidenceArchive']['sha256']:
        raise RuntimeError('Security evidence checksum mismatch')
    assets.update({'publication.json': evidence_path.read_bytes(), 'image-security-evidence.zip': raw})
    assets['SHA256SUMS'] = ''.join(hashlib.sha256(data).hexdigest() + '  ' + name + '\n'
                                  for name, data in assets.items()).encode()
    body = NOTES_PATH.read_text()
    # Existing releases may update notes after validating the same source and assets.
    releases = request('/releases?per_page=100')
    existing = next((r for r in releases if r['tag_name'] == TAG), None)
    if existing:
        if existing['target_commitish'] != source or not existing['prerelease']:
            raise RuntimeError('Existing release differs; refusing to overwrite')
        release = existing
    else:
        release = request('/releases', 'POST', {'tag_name': TAG, 'target_commitish': source,
            'name': f'ICAN {TAG} (Pre-release)', 'body': body, 'draft': True,
            'prerelease': True, 'make_latest': 'false'})
    uploaded = {a['name']: a for a in request(f"/releases/{release['id']}/assets?per_page=100")}
    for name, data in assets.items():
        expected = 'sha256:' + hashlib.sha256(data).hexdigest()
        if name in uploaded:
            if uploaded[name].get('digest') != expected:
                raise RuntimeError('Existing release asset differs: ' + name)
            continue
        if not release['draft']:
            raise RuntimeError('Published release asset missing; refusing to alter it')
        asset = request(f"/releases/{release['id']}/assets?name={urllib.parse.quote(name)}",
                        'POST', data, binary=True)
        if asset.get('digest') != expected:
            raise RuntimeError('Uploaded asset checksum mismatch: ' + name)
    if release['draft']:
        release = request(f"/releases/{release['id']}", 'PATCH',
                          {'draft': False, 'prerelease': True, 'make_latest': 'false'})
    if release.get('body') != body:
        release = request(f"/releases/{release['id']}", 'PATCH', {'body': body})
    final_tag = request('/git/ref/tags/' + TAG)
    if final_tag['object']['sha'] != source or release['draft'] or not release['prerelease']:
        raise RuntimeError('Final release state mismatch')
    print(release['html_url'])
    with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as summary:
        summary.write(f"Published [{TAG}]({release['html_url']}) as a pre-release.\n\n"
                      f"Source: `{source}`\n\nImage: `{image}`\n\n"
                      "Existing checked image reused; no compilation or server deployment.\n")


if __name__ == '__main__':
    main()
