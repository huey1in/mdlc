param(
    [switch]$Race
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectRoot
try {
    $listedFiles = @(& git ls-files --cached --others --exclude-standard -- '*.go')
    if ($LASTEXITCODE -ne 0) {
        throw 'Failed to list Go source files.'
    }
    $goFiles = @($listedFiles | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf })
    if ($goFiles.Count -eq 0) {
        throw 'No Go source files found.'
    }
    $unformatted = @(& gofmt -l @goFiles)
    if ($LASTEXITCODE -ne 0) {
        throw 'gofmt failed.'
    }
    if ($unformatted.Count -ne 0) {
        throw ('Run gofmt on: ' + ($unformatted -join ', '))
    }

    & go vet ./...
    if ($LASTEXITCODE -ne 0) {
        throw 'go vet failed.'
    }
    $testArgs = @('test', '-buildvcs=false')
    if ($Race) {
        $testArgs += '-race'
    }
    $testArgs += './...'
    & go @testArgs
    if ($LASTEXITCODE -ne 0) {
        throw 'go test failed.'
    }
    & go build ./...
    if ($LASTEXITCODE -ne 0) {
        throw 'go build failed.'
    }
    Write-Output 'All project checks passed.'
} finally {
    Pop-Location
}
