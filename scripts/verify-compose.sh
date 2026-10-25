#!/bin/sh
# Disposable CI deployment of the actual Compose file, with synthetic data only.
set -eu
ICAN_COMPOSE_TMP=$(mktemp -d)
ICAN_COMPOSE_PROJECT="ican-compose-${GITHUB_RUN_ID:-local}-$$"
compose() { docker compose -p "$ICAN_COMPOSE_PROJECT" --env-file "$ICAN_COMPOSE_TMP/fixture.env" -f docker-compose.yml "$@"; }
cleanup() {
  compose down --volumes >/dev/null 2>&1 || true
  # The data directory belongs to the container user; remove only this CI fixture.
  docker run --rm --network none --read-only --cap-drop ALL --user 10001:10001 --mount "type=bind,source=$ICAN_COMPOSE_TMP/data,target=/fixture" --entrypoint node ican:verify -e 'const fs=require("node:fs");for(const n of fs.readdirSync("/fixture"))fs.rmSync("/fixture/"+n,{recursive:true,force:true})' >/dev/null 2>&1 || true
  rm -rf "$ICAN_COMPOSE_TMP"
}
trap cleanup EXIT INT TERM
mkdir "$ICAN_COMPOSE_TMP/data"
chmod 700 "$ICAN_COMPOSE_TMP/data"
ICAN_REVIEWED_ID=$(docker image inspect ican:verify --format '{{.Id}}')
python3 - "$ICAN_COMPOSE_TMP" "$ICAN_REVIEWED_ID" <<'PY'
import base64, os, pathlib, secrets, socket, sys
root = pathlib.Path(sys.argv[1])
with socket.socket() as sock:
    sock.bind(('127.0.0.1', 0))
    port = sock.getsockname()[1]
values = {'ICAN_IMAGE': sys.argv[2], 'ICAN_DATA_DIR': str(root / 'data'), 'ICAN_HOST_PORT': str(port),
          'PUBLIC_ORIGIN': 'http://127.0.0.1:3000', 'ADMIN_USERNAME': 'compose-fixture-admin',
          'ADMIN_PASSWORD': secrets.token_urlsafe(32), 'JWT_SECRET': secrets.token_urlsafe(48),
          'DATA_ENCRYPTION_KEY': base64.b64encode(secrets.token_bytes(32)).decode(),
          'AUTH_ATTEMPTS_PER_MINUTE': '1000', 'MEDIA_LINK_TTL_SECONDS': '60',
          'ICAN_PIDS_LIMIT': '256', 'ICAN_CPUS': '1.6', 'ICAN_MEMORY': '1536m',
          'ICAN_TMP_BYTES': '67108864', 'ICAN_CACHE_BYTES': '33554432'}
with (root / 'fixture.env').open('w') as out:
    out.write('\n'.join(k + '=' + v for k, v in values.items()) + '\n')
os.chmod(root / 'fixture.env', 0o600)
(root / 'port').write_text(str(port))
PY
# Directory ownership initialization is confined to an empty fixture directory.
docker run --rm --network none --read-only --cap-drop ALL --cap-add CHOWN --security-opt no-new-privileges --user 0:0 --mount "type=bind,source=$ICAN_COMPOSE_TMP/data,target=/fixture" --entrypoint node ican:verify -e 'require("node:fs").chownSync("/fixture",10001,10001)'
compose config --quiet
compose create --no-build app
ICAN_COMPOSE_CONTAINER=$(compose ps -aq app)
python3 scripts/check-container.py "$ICAN_COMPOSE_CONTAINER" --image-id "$ICAN_REVIEWED_ID" --data-dir "$ICAN_COMPOSE_TMP/data" --host-port "$(cat "$ICAN_COMPOSE_TMP/port")" --cpus 1.6 --memory 1536m > compose-policy.json
# Inspect another operator-selected size, then restore the requested smoke-test size.
env ICAN_CPUS=2 ICAN_MEMORY=2g docker compose -p "$ICAN_COMPOSE_PROJECT" --env-file "$ICAN_COMPOSE_TMP/fixture.env" -f docker-compose.yml create --no-build app
ICAN_ALTERNATE_CONTAINER=$(compose ps -aq app)
python3 scripts/check-container.py "$ICAN_ALTERNATE_CONTAINER" --image-id "$ICAN_REVIEWED_ID" --data-dir "$ICAN_COMPOSE_TMP/data" --host-port "$(cat "$ICAN_COMPOSE_TMP/port")" --cpus 2 --memory 2g > "$ICAN_COMPOSE_TMP/alternate-policy.json"
python3 - "$ICAN_COMPOSE_TMP/alternate-policy.json" <<'PYALTERNATE'
import json, pathlib, sys
path = pathlib.Path('compose-policy.json')
record = json.loads(path.read_text())
record['alternateConfiguration'] = json.loads(pathlib.Path(sys.argv[1]).read_text())
path.write_text(json.dumps(record, indent=2) + '\n')
PYALTERNATE
compose create --no-build app
ICAN_COMPOSE_CONTAINER=$(compose ps -aq app)
python3 scripts/check-container.py "$ICAN_COMPOSE_CONTAINER" --image-id "$ICAN_REVIEWED_ID" --data-dir "$ICAN_COMPOSE_TMP/data" --host-port "$(cat "$ICAN_COMPOSE_TMP/port")" --cpus 1.6 --memory 1536m >/dev/null
compose start app
docker exec -i "$ICAN_COMPOSE_CONTAINER" node < scripts/image-smoke.mjs
# The host-published endpoint must respond; no public supplier calls are made.
python3 - "$ICAN_COMPOSE_TMP/port" <<'PY'
import pathlib, sys, urllib.request
port = pathlib.Path(sys.argv[1]).read_text()
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
with opener.open('http://127.0.0.1:' + port + '/api/health', timeout=10) as response:
    assert response.status == 200
print('Actual Compose loopback publication and application smoke passed')
PY
