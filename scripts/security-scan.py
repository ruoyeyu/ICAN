from pathlib import Path
import json,os,subprocess,urllib.request,concurrent.futures,re,datetime,shutil
root=Path.cwd();out=root/'docs/security/evidence';out.mkdir(parents=True,exist_ok=True)
# bun.lock is JSON-with-trailing-commas, produced by the inspected package manager.
lock=json.loads(re.sub(r',\s*([}\]])',r'\1',(root/'web/bun.lock').read_text()))
queries=[];seen=set()
for p in lock['packages'].values():
 full=p[0];name,version=full.rsplit('@',1)
 if not name or not version:continue
 key=('npm',name,version)
 if key not in seen:queries.append({'package':{'ecosystem':'npm','name':name},'version':version});seen.add(key)
e=os.environ.copy();e['GOTOOLCHAIN']='local';go=shutil.which('go');govulncheck=shutil.which('govulncheck')
if not go or not govulncheck: raise SystemExit('Install the reviewed Go toolchain and govulncheck first; this script installs nothing.')
p=subprocess.run([go,'list','-m','-json','all'],env=e,capture_output=True,text=True,check=True)
decoder=json.JSONDecoder();s=p.stdout
while s.strip():
 v,idx=decoder.raw_decode(s.lstrip());s=s.lstrip()[idx:]
 if v.get('Version'): queries.append({'package':{'ecosystem':'Go','name':v['Path']},'version':v['Version']})
(out/'resolved-dependencies.json').write_text(json.dumps(queries,indent=2))
results=[]
for i in range(0,len(queries),100):
 q=queries[i:i+100];r=urllib.request.Request('https://api.osv.dev/v1/querybatch',data=json.dumps({'queries':q}).encode(),headers={'Content-Type':'application/json'})
 v=json.load(urllib.request.urlopen(r,timeout=60))
 results.extend({'dependency':d,'vulns':x.get('vulns',[])} for d,x in zip(q,v['results']))
matched=[x for x in results if x['vulns']]
(out/'osv-results.json').write_text(json.dumps({'checkedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'queries':len(queries),'matched':matched},indent=2))
ids=sorted({x['id'] for r in matched for x in r['vulns']})
def fetch(id):return json.load(urllib.request.urlopen('https://api.osv.dev/v1/vulns/'+id,timeout=60))
with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool: details=list(pool.map(fetch,ids))
(out/'osv-details.json').write_text(json.dumps(details,indent=2))
print('queries',len(queries),'matched',len(matched),'IDs',len(ids),flush=True)
for item in matched:print(item['dependency'],[x['id'] for x in item['vulns']],flush=True)
r=subprocess.run([govulncheck,'-json','./...'],env=e,stdout=open(out/'govulncheck.json','w'),stderr=open(out/'govulncheck.stderr','w'));print('govulncheck',r.returncode)
