#!/usr/bin/env python3
"""Exercise an existing deploy image; isolated Compose project, ports and storage."""
import argparse
import hashlib
import hmac
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument('--image', default='hyperbricks-deploy-check:local')
parser.add_argument('--api-port', type=int, default=29090)
parser.add_argument('--runtime-port', type=int, default=28080)
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
project = 'hb-smoke-' + secrets.token_hex(4)
secret = secrets.token_hex(32)
env = dict(os.environ, HB_DEPLOY_SECRET=secret, HB_API_PORT=str(args.api_port),
           HB_RUNTIME_PORTS=f'{args.runtime_port}-{args.runtime_port + 20}',
           HB_BIND_ADDRESS='127.0.0.1')
base = f'http://127.0.0.1:{args.api_port}'

def command(*parts):
    result = subprocess.run(parts, env=env, text=True, stdout=subprocess.PIPE,
                            stderr=subprocess.STDOUT)
    if result.returncode:
        raise RuntimeError(f'{parts[0]} failed:\n{result.stdout}')
    return result.stdout.strip()

def api(path, body=None, signed=True):
    payload = body or b''
    method = 'GET' if body is None else 'POST'
    headers = {}
    if signed:
        stamp, nonce = str(int(time.time())), secrets.token_hex(16)
        canonical = '\n'.join([method, path.split('?')[0], hashlib.sha256(payload).hexdigest(), stamp, nonce])
        headers = {'X-HB-Timestamp': stamp, 'X-HB-Nonce': nonce,
                   'X-HB-Signature': hmac.new(secret.encode(), canonical.encode(), hashlib.sha256).hexdigest()}
    request = urllib.request.Request(base + path, data=body, headers=headers, method=method)
    with urllib.request.urlopen(request, timeout=60) as response:
        return response.read()

def ready():
    for _ in range(60):
        try:
            api('/deploy/status')
            return
        except (OSError, urllib.error.HTTPError):
            time.sleep(0.5)
    raise RuntimeError('Deploy API did not start')

with tempfile.TemporaryDirectory(prefix='hb-docker-smoke-') as directory:
    temp = Path(directory)
    (temp / 'deploy').mkdir()
    override = temp / 'compose.yml'
    override.write_text(f'''services:
  hyperbricks-deploy:
    image: {args.image}
    volumes: !override
      - {temp}/deploy:/opt/hyperbricks/deploy
      - plugin-builds:/opt/hyperbricks/bin/plugins
      - {root}/docker/deploy.hyperbricks.yaml:/opt/hyperbricks/deploy.hyperbricks.yaml:ro
''')
    prefix = ['docker', 'compose', '-p', project, '-f', str(root / 'docker/docker-compose.yml'), '-f', str(override)]
    def compose(*parts):
        return command(*prefix, *parts)
    def run(*parts):
        return compose('exec', '-T', '-u', 'deploy', '-w', '/opt/hyperbricks',
                       '-e', 'HYPERBRICKS_LOCAL_PATH=/opt/hyperbricks-source',
                       'hyperbricks-deploy', *parts)
    try:
        compose('up', '-d', '--no-build')
        ready()
        try:
            api('/deploy/modules', signed=False)
            raise AssertionError('Unsigned API request was accepted')
        except urllib.error.HTTPError as error:
            assert error.code == 401, error.code
        print('PASS: daemon and HMAC rejection', flush=True)
        run('hyperbricks', 'init', '-m', 'docker-proof', '--non-interactive')
        container = compose('ps', '-q', 'hyperbricks-deploy')
        fixture = temp / 'fixture'
        fixture.mkdir()
        (fixture / 'proof.go').write_text('''package main
import ("context"; "github.com/hyperbricks/hyperbricks/pkg/shared")
type Proof struct{}
func (*Proof) Render(any, context.Context) (any, []error) { return "<p>PLUGIN-PROOF</p>", nil }
func Plugin() (shared.PluginRenderer, error) { return &Proof{}, nil }
''')
        (fixture / 'go.mod').write_text('module github.com/hyperbricks/plugins/docker-proof\n\ngo 1.26.1\n\nrequire github.com/hyperbricks/hyperbricks v1.2.3-beta\n')
        (fixture / 'manifest.json').write_text(json.dumps(dict(plugin='github.com/hyperbricks/plugins/docker-proof', source='proof.go',
            binary='DockerProof', version='1.0.0', description='Local Docker smoke test', compatible_hyperbricks=['>=1.2.3-beta'])))
        run('mkdir', '-p', 'plugins/docker-proof/1.0.0')
        command('docker', 'cp', str(fixture) + '/.', container + ':/opt/hyperbricks/plugins/docker-proof/1.0.0/')
        compose('exec', '-T', 'hyperbricks-deploy', 'chown', '-R', 'deploy:deploy', '/opt/hyperbricks/plugins/docker-proof')
        print(run('hyperbricks', 'plugin', 'build', 'docker-proof@1.0.0'), flush=True)
        run('test', '-s', 'bin/plugins/DockerProof@1.0.0.so')
        print('PASS: native plugin built against checkout inside image', flush=True)
        package = run('cat', 'modules/docker-proof/package.hyperbricks.yaml')
        package = package.replace('enabled: []', 'enabled: [DockerProof@1.0.0]')
        (temp / 'package.yaml').write_text(package)
        command('docker', 'cp', str(temp / 'package.yaml'), container + ':/opt/hyperbricks/modules/docker-proof/package.hyperbricks.yaml')
        (temp / 'page.yaml').write_text('''style:
  - type: esbuild
  - entry:
      path: {base: resources, path: proof.css}
  - outfile:
      path: {base: static, path: proof.css}
  - cache: true
  - fingerprint: true
  - enclose: <link rel="stylesheet" href="|">
page:
  - type: hypermedia
  - route: index
  - title: Docker proof
  - head:
      - type: head
      - stylesheet:
          - inherit: style
  - content:
      - type: html
      - value: <h1>DOCKER-HRA-PROOF</h1>
  - plugin:
      - type: plugin
      - plugin: DockerProof@1.0.0
''')
        (temp / 'proof.css').write_text('body { color: rgb(21, 78, 44); }')
        run('sh', '-c', 'rm -f modules/docker-proof/hyperbricks/*.hyperbricks.yaml')
        command('docker', 'cp', str(temp / 'page.yaml'), container + ':/opt/hyperbricks/modules/docker-proof/hyperbricks/page.hyperbricks.yaml')
        command('docker', 'cp', str(temp / 'proof.css'), container + ':/opt/hyperbricks/modules/docker-proof/resources/proof.css')
        compose('exec', '-T', 'hyperbricks-deploy', 'chown', '-R', 'deploy:deploy', '/opt/hyperbricks/modules')
        run('hyperbricks', 'build', '--hra', '-m', 'docker-proof', '--out', '/tmp/proof-build', '--non-interactive')
        archive_path = run('sh', '-c', 'find /tmp/proof-build -name "*.hra" | head -1')
        assert archive_path
        command('docker', 'cp', container + ':' + archive_path, str(temp / 'proof.hra'))
        archive = (temp / 'proof.hra').read_bytes()
        result = json.loads(api('/deploy/v1/modules/docker-proof/releases?build_id=proof-001', archive))
        print('PASS: HRA upload and activation: ' + json.dumps(result), flush=True)
        def check_page():
            status = json.loads(api('/deploy/modules/docker-proof/status'))
            url = f'http://127.0.0.1:{args.runtime_port + status["port"] - 8080}/'
            for _ in range(60):
                try:
                    with urllib.request.urlopen(url, timeout=5) as response:
                        page = response.read().decode()
                        assert response.headers.get('X-Hyperbricks-Render-Error-Count', '0') == '0', page + '\n' + run('sh', '-c', 'cat deploy/docker-proof/logs/* 2>/dev/null | tail -60')
                    assert 'DOCKER-HRA-PROOF' in page and 'PLUGIN-PROOF' in page, page
                    asset = re.search(r'href="(/static/[^\"]+\.css)"', page).group(1)
                    with urllib.request.urlopen(url.rstrip('/') + asset, timeout=5) as response:
                        assert 'color' in response.read().decode()
                    return
                except OSError:
                    time.sleep(0.5)
            raise AssertionError('Deployed page did not start')
        check_page()
        print('PASS: deployed page, Linux plugin, fingerprinted native esbuild CSS', flush=True)
        plugin_hash = run('sha256sum', 'bin/plugins/DockerProof@1.0.0.so').split()[0]
        api('/deploy/modules/docker-proof/stop', b'{}')
        compose('up', '-d', '--no-build', '--force-recreate')
        ready()
        assert run('sha256sum', 'bin/plugins/DockerProof@1.0.0.so').split()[0] == plugin_hash
        status = json.loads(api('/deploy/modules/docker-proof/status'))
        assert status['current'] == 'proof-001'
        assert (temp / 'deploy/docker-proof/hyperbricks.versions.json').exists()
        api('/deploy/modules/docker-proof/restart', b'{}')
        check_page()
        print('PASS: archive/index/plugin persistence after recreation and module restart', flush=True)
        api('/deploy/modules/docker-proof/stop', b'{}')
    finally:
        try:
            print(compose('logs', '--tail=25', 'hyperbricks-deploy'), flush=True)
        finally:
            compose('down', '-v')
