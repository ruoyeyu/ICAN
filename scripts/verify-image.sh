#!/bin/sh
# Isolated CI fixtures only. No production data, supplier calls or registry push.
set -eu
ICAN_TEST_CONTAINER="ican-verify-${GITHUB_RUN_ID:-local}"
ICAN_TEST_VOLUME="ican-verify-data-${GITHUB_RUN_ID:-local}"
ICAN_TEST_ENV=$(mktemp)
cleanup() {
  docker rm -f "$ICAN_TEST_CONTAINER" >/dev/null 2>&1 || true
  docker volume rm "$ICAN_TEST_VOLUME" >/dev/null 2>&1 || true
  rm -f "$ICAN_TEST_ENV"
}
trap cleanup EXIT INT TERM
python3 - "$ICAN_TEST_ENV" <<'PY'
import secrets,base64,sys,os
values={'PUBLIC_ORIGIN':'http://127.0.0.1:3000','ADMIN_USERNAME':'image-fixture-admin','ADMIN_PASSWORD':secrets.token_urlsafe(32),'JWT_SECRET':secrets.token_urlsafe(48),'DATA_ENCRYPTION_KEY':base64.b64encode(secrets.token_bytes(32)).decode(),'AUTH_ATTEMPTS_PER_MINUTE':'1000','MEDIA_LINK_TTL_SECONDS':'60','DATABASE_DSN':'/app/data/verify.db','MEDIA_DIR':'/app/data/media'}
with open(sys.argv[1],'w') as f:f.write('\n'.join(k+'='+v for k,v in values.items())+'\n')
os.chmod(sys.argv[1],0o600)
PY
test "$(docker image inspect ican:verify --format '{{.Config.User}}')" = '10001:10001'
docker volume create "$ICAN_TEST_VOLUME" >/dev/null
docker run -d --name "$ICAN_TEST_CONTAINER" --network none --read-only --cap-drop ALL --security-opt no-new-privileges --pids-limit 256 --memory 1g --cpus 2 --env-file "$ICAN_TEST_ENV" --mount "type=volume,source=$ICAN_TEST_VOLUME,target=/app/data" --tmpfs /tmp:rw,noexec,nosuid,nodev,size=64m,uid=10001,gid=10001 --tmpfs /app/web/.next/cache:rw,noexec,nosuid,nodev,size=32m,uid=10001,gid=10001 ican:verify >/dev/null
docker exec -i "$ICAN_TEST_CONTAINER" node < scripts/image-smoke.mjs
docker restart "$ICAN_TEST_CONTAINER" >/dev/null
docker exec "$ICAN_TEST_CONTAINER" node -e '(async()=>{for(let i=0;i<60;i++){try{const r=await fetch("http://127.0.0.1:3000/api/auth/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({username:"image-fixture-user",password:"fixture-password-only"})});const v=await r.json();if(v.code===0){const headers={Authorization:"Bearer "+v.data.token};const saved=JSON.parse(require("fs").readFileSync("/app/data/template-restart-fixture.json","utf8"));const entry=await(await fetch("http://127.0.0.1:3000/api/v1/channel-templates/"+saved.id,{headers})).json();const config=await(await fetch("http://127.0.0.1:3000/api/v1/user-config",{headers})).json();require("assert/strict").equal(entry.data.revision,saved.revision);require("assert/strict").equal(config.data.modelConfig.channelTemplates[0].id,"personal-fixture");console.log("Persistent account, supplier file and personal template restored after container restart");return}}catch{}await new Promise(r=>setTimeout(r,500))}process.exit(1)})()'
docker exec "$ICAN_TEST_CONTAINER" node -e 'console.log(JSON.stringify({node:process.version,components:process.versions,sharpBundled:false}))'
