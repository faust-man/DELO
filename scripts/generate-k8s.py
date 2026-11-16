"""Render the Linux-compatible kubectl profile. No secrets are generated or saved."""
import argparse
import hashlib
import json
import re
from pathlib import Path

MODULES = ['identity', 'contacts', 'organizations', 'education', 'bookings',
           'subscriptions', 'finance', 'artworks', 'inventory', 'exhibitions',
           'reports', 'audit', 'notifications']
SERVICES = MODULES + ['gateway', 'worker', 'web']
ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--image-prefix', default='delo')
    parser.add_argument('--tag', default='micro')
    parser.add_argument('--namespace', default='delo-micro-local')
    parser.add_argument('--origin', default='http://localhost:8088')
    parser.add_argument('--cookie-secure', choices=['true', 'false'], default='false')
    parser.add_argument('--image', action='append', default=[], metavar='SERVICE=IMAGE',
                        help='Override one service image for an independent release; repeat as needed')
    args = parser.parse_args()
    overrides = {}
    for override in args.image:
        name, separator, value = override.partition('=')
        if separator != '=' or name not in SERVICES + ['admin'] or not value or any(c.isspace() for c in value):
            parser.error('image override must be SERVICE=IMAGE')
        overrides[name] = value
    if not re.fullmatch(r'[a-z0-9]([-a-z0-9]*[a-z0-9])?', args.namespace) or len(args.namespace) > 63:
        parser.error('invalid namespace')
    if not re.fullmatch(r'[\w][\w.-]{0,127}', args.tag, re.ASCII):
        parser.error('invalid image tag')
    dest = ROOT / 'infra/k8s/local'
    dest.mkdir(parents=True, exist_ok=True)
    files = []

    def image(name):
        return overrides.get(name, f'{args.image_prefix}-{name}:{args.tag}')

    def save(name, obj, directory=dest, include=True):
        directory.mkdir(parents=True, exist_ok=True)
        (directory / (name + '.json')).write_text(json.dumps(obj, indent=2) + '\n', encoding='utf-8')
        if include:
            files.append(name + '.json')

    def limits(cpu='250m', memory='128Mi'):
        return {'requests': {'cpu': '25m', 'memory': '32Mi', 'ephemeral-storage': '16Mi'},
                'limits': {'cpu': cpu, 'memory': memory, 'ephemeral-storage': '128Mi'}}

    security = {'allowPrivilegeEscalation': False, 'readOnlyRootFilesystem': True,
                'capabilities': {'drop': ['ALL']}}
    database = {'name': 'DATABASE_URL', 'valueFrom': {'secretKeyRef': {'name': 'delo-runtime', 'key': 'DATABASE_URL'}}}

    def container(name, component=None, command=None):
        c = {'name': name, 'image': image(component or name), 'imagePullPolicy': 'IfNotPresent',
             'resources': limits(), 'securityContext': security,
             'volumeMounts': [{'name': 'tmp', 'mountPath': '/tmp'}]}
        if command:
            c['command'] = command
        return c

    def pod(containers, labels=None):
        return {'metadata': {'labels': labels or {}}, 'spec': {
            'automountServiceAccountToken': False,
            'securityContext': {'runAsNonRoot': True, 'runAsUser': 65532,
                                'seccompProfile': {'type': 'RuntimeDefault'}},
            'containers': containers, 'volumes': [{'name': 'tmp', 'emptyDir': {'sizeLimit': '64Mi'}}],
            'terminationGracePeriodSeconds': 30}}

    save('namespace', {'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': args.namespace}})
    save('runtime', {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'delo-config'},
                     'data': {'APP_ORIGIN': args.origin, 'COOKIE_SECURE': args.cookie_secure,
                              'GOMEMLIMIT': '96MiB', 'GOMAXPROCS': '1'}})
    for name in SERVICES:
        c = container(name)
        labels = {'app': 'delo-' + name, 'delo-tier': 'module' if name in MODULES else name}
        if name in MODULES + ['worker']:
            c['env'] = [database]
            c['envFrom'] = [{'configMapRef': {'name': 'delo-config'}}]
            labels['delo-db-client'] = 'true'
        if name == 'web':
            c['resources'] = limits('200m', '96Mi')
            c['volumeMounts'].append({'name': 'nginx', 'mountPath': '/etc/nginx/nginx.conf', 'subPath': 'nginx.conf'})
        if name != 'worker':
            c['ports'] = [{'containerPort': 8080}]
            for probe, path in [('readinessProbe', '/readyz'), ('livenessProbe', '/healthz')]:
                c[probe] = {'httpGet': {'path': '/' if name == 'web' else path, 'port': 8080},
                            'periodSeconds': 10, 'timeoutSeconds': 2, 'failureThreshold': 3}
            c['startupProbe'] = {'httpGet': {'path': '/' if name == 'web' else '/healthz', 'port': 8080},
                                 'periodSeconds': 2, 'failureThreshold': 30}
        template = pod([c], labels)
        if name in MODULES + ['worker']:
            init = container('wait-migrations', 'admin', ['/app/admin', 'wait-migrations'])
            init['env'] = [database]
            template['spec']['initContainers'] = [init]
        if name == 'web':
            template['spec']['securityContext']['runAsUser'] = 101
            template['spec']['volumes'].append({'name': 'nginx', 'configMap': {'name': 'delo-nginx'}})
        save(name, {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': {'name': 'delo-' + name},
                    'spec': {'replicas': 1, 'selector': {'matchLabels': {'app': 'delo-' + name}},
                             'template': template}})
        if name != 'worker':
            save(name + '-service', {'apiVersion': 'v1', 'kind': 'Service', 'metadata': {'name': 'delo-' + name},
                                     'spec': {'selector': {'app': 'delo-' + name}, 'ports': [{'port': 8080, 'targetPort': 8080}]}})

    save('network', {'apiVersion': 'networking.k8s.io/v1', 'kind': 'NetworkPolicy',
                     'metadata': {'name': 'delo-ingress'},
                     'spec': {'podSelector': {}, 'policyTypes': ['Ingress'], 'ingress': []}})
    for name, selector, caller, port in [
            ('modules', {'delo-tier': 'module'}, {'app': 'delo-gateway'}, 8080),
            ('gateway', {'app': 'delo-gateway'}, {'app': 'delo-web'}, 8080),
            ('database', {'app': 'delo-db'}, {'delo-db-client': 'true'}, 5432)]:
        save('network-' + name, {'apiVersion': 'networking.k8s.io/v1', 'kind': 'NetworkPolicy',
             'metadata': {'name': 'delo-' + name}, 'spec': {'podSelector': {'matchLabels': selector},
             'policyTypes': ['Ingress'], 'ingress': [{'from': [{'podSelector': {'matchLabels': caller}}],
             'ports': [{'protocol': 'TCP', 'port': port}]}]}})
    (dest / 'postgres.yaml').write_text((ROOT / 'infra/local/postgres.yaml').read_text(encoding='utf-8'), encoding='utf-8')
    files.append('postgres.yaml')
    nginx = (ROOT / 'frontend/nginx.conf').read_text(encoding='utf-8').replace('delo-api:8080', 'delo-gateway:8080')
    save('nginx', {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'delo-nginx'}, 'data': {'nginx.conf': nginx}})
    revision = hashlib.sha256((image('admin') + ''.join(
        path.read_text(encoding='utf-8') for path in sorted((ROOT / 'backend/db/migrations').glob('*.sql')))).encode()).hexdigest()[:12]
    for name in ['migrate', 'bootstrap']:
        c = container(name, 'admin', ['/app/admin', name])
        c['env'] = [database]
        if name == 'bootstrap':
            c['env'].append({'name': 'BOOTSTRAP_PASSWORD', 'valueFrom': {'secretKeyRef': {'name': 'delo-bootstrap', 'key': 'BOOTSTRAP_PASSWORD'}}})
        template = pod([c], {'delo-db-client': 'true'})
        template['spec']['restartPolicy'] = 'Never'
        job = {'apiVersion': 'batch/v1', 'kind': 'Job',
               'metadata': {'name': 'delo-' + name + '-' + revision},
               'spec': {'backoffLimit': 0, 'activeDeadlineSeconds': 180, 'template': template}}
        save(name, job, ROOT / 'infra/k8s/jobs', include=False)
        if name == 'migrate':
            save(name, job)
    (dest / 'kustomization.yaml').write_text('apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n' + ''.join('  - ' + f + '\n' for f in files), encoding='utf-8')


if __name__ == '__main__':
    main()
