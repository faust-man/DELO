param([switch]$Build,[switch]$Bootstrap,[string]$Namespace='delo-local')
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
Set-Location (Split-Path $PSScriptRoot)
function Check { if ($LASTEXITCODE -ne 0) { throw 'Команда завершилась ошибкой' } }
if ((kubectl config current-context) -ne 'docker-desktop') { throw 'Локальный запуск разрешён только в docker-desktop' }
$helm=Join-Path (Get-Location) '.tools/helm/windows-amd64/helm.exe'
if (!(Test-Path $helm)) { $helm='helm' }
if ($Build) { docker build -t delo-api:dev backend; Check; docker build -t delo-web:dev frontend; Check }
$apiHash=(docker image inspect delo-api:dev --format '{{.Id}}').Split(':')[-1].Substring(0,12); Check
$webHash=(docker image inspect delo-web:dev --format '{{.Id}}').Split(':')[-1].Substring(0,12); Check
$apiImage="delo-api:dev-$apiHash"; $webImage="delo-web:dev-$webHash"
docker tag delo-api:dev $apiImage; Check
docker tag delo-web:dev $webImage; Check
kubectl create namespace $Namespace --dry-run=client -o yaml | kubectl apply -f -; Check
kubectl get secret delo-runtime -n $Namespace *> $null
if ($LASTEXITCODE -ne 0) {
 $password=[Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(24))
 @{apiVersion='v1';kind='Secret';metadata=@{name='delo-db';namespace=$Namespace};stringData=@{password=$password}} | ConvertTo-Json -Depth 8 | kubectl apply -f -; Check
 @{apiVersion='v1';kind='Secret';metadata=@{name='delo-runtime';namespace=$Namespace};stringData=@{DATABASE_URL="postgres://delo:$password@delo-db:5432/delo?sslmode=disable"}} | ConvertTo-Json -Depth 8 | kubectl apply -f -; Check
 $password=$null
}
kubectl apply -n $Namespace -f infra/local/postgres.yaml; Check
kubectl rollout status -n $Namespace statefulset/delo-db --timeout=180s; Check
& $helm upgrade --install delo infra/helm/delo -n $Namespace --set-string "apiImage=$apiImage" --set-string "webImage=$webImage" --wait --timeout 5m; Check
if ($Bootstrap) {
 $secure=Read-Host 'Новый пароль owner@demo.local (минимум 12 символов)' -AsSecureString
 $plain=[System.Net.NetworkCredential]::new('', $secure).Password
 if ($plain.Length -lt 12) { throw 'Пароль слишком короткий' }
 # Secret passed only on stdin; never in command arguments or tracked files.
 @{apiVersion='v1';kind='Secret';metadata=@{name='delo-bootstrap';namespace=$Namespace};stringData=@{BOOTSTRAP_PASSWORD=$plain}} | ConvertTo-Json -Depth 8 | kubectl apply -f -; Check
 $plain=$null
 try {
  $job=@{apiVersion='batch/v1';kind='Job';metadata=@{name=('delo-bootstrap-'+[DateTimeOffset]::UtcNow.ToUnixTimeSeconds());namespace=$Namespace};spec=@{backoffLimit=0;ttlSecondsAfterFinished=60;template=@{spec=@{restartPolicy='Never';automountServiceAccountToken=$false;containers=@(@{name='bootstrap';image=$apiImage;imagePullPolicy='IfNotPresent';command=@('/app/admin','bootstrap');envFrom=@(@{secretRef=@{name='delo-bootstrap'}},@{secretRef=@{name='delo-runtime'}})})}}}}
  $job | ConvertTo-Json -Depth 20 | kubectl apply -f -; Check
  kubectl wait -n $Namespace --for=condition=complete "job/$($job.metadata.name)" --timeout=90s; Check
 } finally { kubectl delete secret delo-bootstrap -n $Namespace --ignore-not-found }
}
Write-Host 'Откройте http://localhost:8088. Остановка port-forward: Ctrl+C.'
kubectl port-forward -n $Namespace service/delo-web 8088:8080
