"""Verified scanner, final local Docker image, complete result plus strict CI gate."""
import hashlib, io, json, os, pathlib, subprocess, tarfile, tempfile, urllib.request
URL='https://github.com/aquasecurity/trivy/releases/download/v0.76.1/trivy_0.76.1_Linux-64bit.tar.gz'
SHA256='c7cbe3137fb8a48a17574bede3dd6beb16742d1e9772929940fc90082c39d9c0'
blob=urllib.request.urlopen(URL,timeout=60).read()
if hashlib.sha256(blob).hexdigest()!=SHA256:raise SystemExit('Scanner checksum mismatch')
with tempfile.TemporaryDirectory(prefix='ican-scanner-') as tmp:
 with tarfile.open(fileobj=io.BytesIO(blob)) as archive:archive.extract(archive.getmember('trivy'),tmp,filter='data')
 subprocess.run([tmp+'/trivy','image','--image-src','docker','--scanners','vuln','--format','json','--output','image-vulnerabilities.json','ican:verify'],check=True)
 subprocess.run([tmp+'/trivy','convert','--format','cyclonedx','--output','image-sbom.cdx.json','image-vulnerabilities.json'],check=True)
report=json.loads(pathlib.Path('image-vulnerabilities.json').read_text())
high=[]
for result in report.get('Results',[]):
 for item in result.get('Vulnerabilities',[]):
  if item.get('Severity') in ['HIGH','CRITICAL']:high.append({'target':result.get('Target'),'id':item.get('VulnerabilityID'),'package':item.get('PkgName'),'installed':item.get('InstalledVersion'),'fixed':item.get('FixedVersion'),'severity':item.get('Severity')})
print(json.dumps({'highOrCriticalMatches':high},indent=2))
if high:raise SystemExit('Image review gate blocked; matches require contextual review, not an exploit-count claim.')
