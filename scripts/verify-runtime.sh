#!/bin/sh
# Isolated fixture network; no supplier APIs, real credentials or production data.
set -eu
ICAN_RUNTIME_TMP=$(mktemp -d)
ICAN_RUNTIME_NETWORK="ican-runtime-${GITHUB_RUN_ID:-local}"
ICAN_TLS_CONTAINER="ican-tls-${GITHUB_RUN_ID:-local}"
cleanup() {
  docker rm -f "$ICAN_TLS_CONTAINER" >/dev/null 2>&1 || true
  docker network rm "$ICAN_RUNTIME_NETWORK" >/dev/null 2>&1 || true
  rm -rf "$ICAN_RUNTIME_TMP"
}
trap cleanup EXIT INT TERM
chmod 755 "$ICAN_RUNTIME_TMP"
g++ -std=c++17 -O0 -fno-builtin scripts/native-regression.cc -o "$ICAN_RUNTIME_TMP/native-check" -lresolv
: > runtime-regressions.log
for mode in allocation resolver; do
  docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --mount "type=bind,source=$ICAN_RUNTIME_TMP/native-check,target=/native-check,readonly" --entrypoint /native-check ican:verify "$mode" >> runtime-regressions.log 2>&1
done
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=ican-tls-fixture -addext subjectAltName=DNS:ican-tls-fixture -keyout "$ICAN_RUNTIME_TMP/key.pem" -out "$ICAN_RUNTIME_TMP/cert.pem" >/dev/null 2>&1
# This is a throwaway fixture key, never a production secret or image layer.
chmod 644 "$ICAN_RUNTIME_TMP/key.pem" "$ICAN_RUNTIME_TMP/cert.pem"
docker network create --internal "$ICAN_RUNTIME_NETWORK" >/dev/null
docker run -d --name "$ICAN_TLS_CONTAINER" --network "$ICAN_RUNTIME_NETWORK" --network-alias ican-tls-fixture --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --mount "type=bind,source=$ICAN_RUNTIME_TMP,target=/fixture,readonly" --entrypoint /usr/local/bin/node ican:verify -e 'const fs=require("node:fs");require("node:https").createServer({key:fs.readFileSync("/fixture/key.pem"),cert:fs.readFileSync("/fixture/cert.pem")},(req,res)=>res.end("fixture-only")).listen(9443,"0.0.0.0")' >/dev/null
docker run --rm --network "$ICAN_RUNTIME_NETWORK" --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 -e NODE_EXTRA_CA_CERTS=/fixture/cert.pem --mount "type=bind,source=$ICAN_RUNTIME_TMP/cert.pem,target=/fixture/cert.pem,readonly" --entrypoint /usr/local/bin/node ican:verify -e '(async()=>{const assert=require("node:assert/strict");for(let i=0;i<40;i++){try{const a=await require("node:dns/promises").lookup("ican-tls-fixture",{all:true});assert.ok(a.length);const r=await fetch("https://ican-tls-fixture:9443");assert.equal(await r.text(),"fixture-only");console.log("isolated DNS, TLS certificate verification and HTTP fetch passed");return}catch(e){if(i===39)throw e;await new Promise(r=>setTimeout(r,250))}}})().catch(e=>{console.error(e.code||e.name);process.exit(1)})' >> runtime-regressions.log 2>&1
docker run --rm --network "$ICAN_RUNTIME_NETWORK" --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --entrypoint /usr/local/bin/node ican:verify -e 'fetch("https://ican-tls-fixture:9443").then(()=>process.exit(1)).catch(e=>{if(!["DEPTH_ZERO_SELF_SIGNED_CERT","SELF_SIGNED_CERT_IN_CHAIN"].includes(e.cause?.code))process.exit(1);console.log("untrusted TLS certificate rejected")})' >> runtime-regressions.log 2>&1
cat runtime-regressions.log
