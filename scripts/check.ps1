param([string]$TestDatabaseUrl=$env:TEST_DATABASE_URL)
$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot)
$go=Join-Path (Get-Location) '.tools/go/bin/go.exe';if (!(Test-Path $go)) {$go='go'}
if (!$TestDatabaseUrl) { throw 'Нужен TEST_DATABASE_URL изолированной PostgreSQL; интеграционные тесты нельзя молча пропускать' }
$env:TEST_DATABASE_URL=$TestDatabaseUrl
Push-Location backend
try { & $go test -count=1 -v ./...; if ($LASTEXITCODE) {throw 'Backend tests failed'}; & $go vet ./...; if ($LASTEXITCODE) {throw 'go vet failed'} } finally {Pop-Location}
Push-Location frontend
try {npm ci; if ($LASTEXITCODE) {throw 'npm ci failed'};npm test;if ($LASTEXITCODE) {throw 'Frontend tests failed'};npm run build;if ($LASTEXITCODE) {throw 'Build failed'};npm audit --audit-level=high;if ($LASTEXITCODE) {throw 'Audit failed'}} finally {Pop-Location}
