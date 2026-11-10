param([switch]$Build,[switch]$Bootstrap,[string]$Namespace='delo-micro-local')
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
Set-Location (Split-Path $PSScriptRoot)
if ($Namespace -ne 'delo-micro-local') { throw 'Для другого namespace сначала подготовьте отдельный Kustomize overlay' }
function Check { if ($LASTEXITCODE -ne 0) { throw 'Команда завершилась ошибкой' } }
if ((kubectl config current-context) -ne 'docker-desktop') { throw 'Этот локальный профиль разрешён только в docker-desktop' }
if ($Build) {
 foreach ($service in @('identity','contacts','organizations','education','bookings','subscriptions','finance','artworks','inventory','exhibitions','reports','audit','notifications','gateway','worker')) {
  docker build --target service --build-arg "SERVICE=$service" -t "delo-${service}:micro" backend; Check
 }
 docker build --target admin -t delo-admin:micro backend; Check
 docker build -t delo-web:micro frontend; Check
}
kubectl create namespace $Namespace --dry-run=client -o yaml | kubectl apply -f -; Check
kubectl get secret delo-runtime -n $Namespace *> $null
if ($LASTEXITCODE -ne 0) {
 $password=[Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(24))
 @{apiVersion='v1';kind='Secret';metadata=@{name='delo-db';namespace=$Namespace};stringData=@{password=$password}} | ConvertTo-Json -Depth 8 | kubectl apply -f -; Check
 @{apiVersion='v1';kind='Secret';metadata=@{name='delo-runtime';namespace=$Namespace};stringData=@{DATABASE_URL="postgres://delo:$password@delo-db:5432/delo?sslmode=disable"}} | ConvertTo-Json -Depth 8 | kubectl apply -f -; Check
 $password=$null
}
kubectl apply -n $Namespace -k infra/k8s/local; Check
if ($Build) { kubectl rollout restart deployment -n $Namespace; Check }
kubectl rollout status deployment -n $Namespace --timeout=300s; Check
if ($Bootstrap) {
 $secure=Read-Host 'Новый пароль owner@demo.local (минимум 12 символов)' -AsSecureString
 $plain=[System.Net.NetworkCredential]::new('', $secure).Password
 if ($plain.Length -lt 12) { throw 'Пароль слишком короткий' }
 @{apiVersion='v1';kind='Secret';metadata=@{name='delo-bootstrap';namespace=$Namespace};stringData=@{BOOTSTRAP_PASSWORD=$plain}} | ConvertTo-Json -Depth 8 | kubectl apply -f -; Check
 $plain=$null
 try {
  kubectl delete -f infra/k8s/jobs/bootstrap.json -n $Namespace --ignore-not-found --wait=true; Check
  kubectl apply -n $Namespace -f infra/k8s/jobs/bootstrap.json; Check
  kubectl wait -n $Namespace --for=condition=complete -f infra/k8s/jobs/bootstrap.json --timeout=180s; Check
 } finally { kubectl delete secret delo-bootstrap -n $Namespace --ignore-not-found }
}
Write-Host 'Откройте http://localhost:8088. Остановка port-forward: Ctrl+C.'
kubectl port-forward -n $Namespace service/delo-web 8088:8080
