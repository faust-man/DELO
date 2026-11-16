"""Check service image boundaries and Kubernetes container resource contracts."""
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
local = ROOT / 'infra/k8s/local'
deployments = [json.loads(p.read_text(encoding='utf-8')) for p in local.glob('*.json')]
deployments = [obj for obj in deployments if obj['kind'] == 'Deployment']
assert len(deployments) == 16, 'expected 13 modules + gateway + worker + web'
images = set()
for obj in deployments:
    name = obj['metadata']['name'].removeprefix('delo-')
    spec = obj['spec']['template']['spec']
    app = spec['containers'][0]
    assert app['image'] not in images, f'{name}: shared runtime image'
    images.add(app['image'])
    assert not app.get('command'), f'{name}: entrypoint overridden'
    assert all(env['name'] != 'SERVICE_NAME' for env in app.get('env', []))
    if name != 'web':
        assert (ROOT / 'backend/cmd' / name / 'main.go').is_file()
    if name in ['web', 'gateway']:
        assert not app.get('envFrom') and not app.get('env'), f'{name}: must not receive database credentials'
    else:
        assert spec['initContainers'][0]['command'] == ['/app/admin', 'wait-migrations']
    for c in spec['containers'] + spec.get('initContainers', []):
        for field in ['requests', 'limits']:
            assert {'cpu', 'memory', 'ephemeral-storage'} <= c['resources'][field].keys(), f'{name}: missing {field}'
        assert c['securityContext']['readOnlyRootFilesystem']
        assert not c['securityContext']['allowPrivilegeEscalation']
        assert c['securityContext']['capabilities']['drop'] == ['ALL']
for path in (ROOT / 'infra/k8s/jobs').glob('*.json'):
    job = json.loads(path.read_text(encoding='utf-8'))
    assert '-admin:' in job['spec']['template']['spec']['containers'][0]['image']
    assert job['spec']['activeDeadlineSeconds'] <= 180
assert (local / 'migrate.json').read_bytes() == (ROOT / 'infra/k8s/jobs/migrate.json').read_bytes()
print('OK: 16 unique runtime images, init migration barriers, resource/security contracts and admin jobs')
