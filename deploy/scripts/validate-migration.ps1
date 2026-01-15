# CodeSwitch SSOT Migration Validation Script
# Purpose: Validate Phase 1 migration implementation
# Usage: .\validate-migration.ps1

$ErrorActionPreference = "Stop"

Write-Host "===========================================" -ForegroundColor Cyan
Write-Host "  CodeSwitch Phase 1 Migration Validation  " -ForegroundColor Cyan
Write-Host "===========================================" -ForegroundColor Cyan
Write-Host ""

# Test counter
$script:TestsPassed = 0
$script:TestsFailed = 0
$script:TotalTests = 0

function Test-Step {
    param(
        [string]$Name,
        [scriptblock]$Action
    )

    $script:TotalTests++
    Write-Host "[TEST $script:TotalTests] $Name" -ForegroundColor Yellow

    try {
        & $Action
        Write-Host "  ✓ PASSED" -ForegroundColor Green
        $script:TestsPassed++
        return $true
    }
    catch {
        Write-Host "  ✗ FAILED: $_" -ForegroundColor Red
        $script:TestsFailed++
        return $false
    }
}

# Get project root
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Resolve-Path (Join-Path $scriptDir "../..")
$servicesDir = Join-Path $projectRoot "services"

Write-Host "Project Root: $projectRoot" -ForegroundColor Gray
Write-Host ""

# ============================================================================
# Phase 1: File Structure Validation
# ============================================================================

Write-Host "Phase 1: Validating File Structure..." -ForegroundColor Cyan
Write-Host ""

Test-Step "Schema file exists (deploy/sqlite/schema_v2.sql)" {
    $schemaPath = Join-Path $projectRoot "deploy/sqlite/schema_v2.sql"
    if (-not (Test-Path $schemaPath)) {
        throw "Schema file not found: $schemaPath"
    }

    $content = Get-Content $schemaPath -Raw
    if ($content -notmatch "provider_config") {
        throw "Schema does not contain provider_config table"
    }
    if ($content -notmatch "provider_health") {
        throw "Schema does not contain provider_health table"
    }
    if ($content -notmatch "proxy_control") {
        throw "Schema does not contain proxy_control table"
    }
    if ($content -notmatch "proxy_live_backup") {
        throw "Schema does not contain proxy_live_backup table"
    }
}

Test-Step "Migration tool exists (services/migration/ssot_migration.go)" {
    $migrationPath = Join-Path $servicesDir "migration/ssot_migration.go"
    if (-not (Test-Path $migrationPath)) {
        throw "Migration tool not found: $migrationPath"
    }

    $content = Get-Content $migrationPath -Raw
    if ($content -notmatch "type SSOTMigration") {
        throw "SSOTMigration type not found"
    }
    if ($content -notmatch "func.*Execute") {
        throw "Execute function not found"
    }
}

Test-Step "Auto-migrate helper exists (services/migration/auto_migrate.go)" {
    $autoMigratePath = Join-Path $servicesDir "migration/auto_migrate.go"
    if (-not (Test-Path $autoMigratePath)) {
        throw "Auto-migrate helper not found: $autoMigratePath"
    }

    $content = Get-Content $autoMigratePath -Raw
    if ($content -notmatch "func AutoMigrate") {
        throw "AutoMigrate function not found"
    }
}

Test-Step "Provider Service V2 exists (services/providerservice_v2.go)" {
    $serviceV2Path = Join-Path $servicesDir "providerservice_v2.go"
    if (-not (Test-Path $serviceV2Path)) {
        throw "ProviderServiceV2 not found: $serviceV2Path"
    }

    $content = Get-Content $serviceV2Path -Raw
    if ($content -notmatch "type ProviderServiceV2") {
        throw "ProviderServiceV2 type not found"
    }
    if ($content -notmatch "func.*LoadProviders") {
        throw "LoadProviders method not found"
    }
    if ($content -notmatch "func.*SaveProviders") {
        throw "SaveProviders method not found"
    }
}

Test-Step "Migration tests exist (services/migration/migration_test.go)" {
    $testPath = Join-Path $servicesDir "migration/migration_test.go"
    if (-not (Test-Path $testPath)) {
        throw "Migration tests not found: $testPath"
    }

    $content = Get-Content $testPath -Raw
    if ($content -notmatch "TestMigration_EmptyConfig") {
        throw "Empty config test not found"
    }
    if ($content -notmatch "TestMigration_WithExistingConfig") {
        throw "Existing config test not found"
    }
    if ($content -notmatch "TestMigration_Rollback") {
        throw "Rollback test not found"
    }
}

Test-Step "Provider Service V2 tests exist (services/providerservice_v2_test.go)" {
    $testPath = Join-Path $servicesDir "providerservice_v2_test.go"
    if (-not (Test-Path $testPath)) {
        throw "ProviderServiceV2 tests not found: $testPath"
    }

    $content = Get-Content $testPath -Raw
    if ($content -notmatch "TestProviderServiceV2_SaveAndLoad") {
        throw "Save and load test not found"
    }
    if ($content -notmatch "TestProviderServiceV2_BackwardCompatibility") {
        throw "Backward compatibility test not found"
    }
}

Write-Host ""

# ============================================================================
# Phase 2: Schema Validation
# ============================================================================

Write-Host "Phase 2: Validating Schema Design..." -ForegroundColor Cyan
Write-Host ""

$schemaPath = Join-Path $projectRoot "deploy/sqlite/schema_v2.sql"
$schemaContent = Get-Content $schemaPath -Raw

Test-Step "Schema has proper table definitions" {
    $requiredTables = @(
        "provider_config",
        "provider_health",
        "proxy_control",
        "proxy_live_backup",
        "schema_version"
    )

    foreach ($table in $requiredTables) {
        if ($schemaContent -notmatch "CREATE TABLE.*$table") {
            throw "Table definition missing: $table"
        }
    }
}

Test-Step "Schema has proper indexes" {
    $requiredIndexes = @(
        "idx_provider_platform",
        "idx_provider_priority",
        "idx_health_state"
    )

    foreach ($index in $requiredIndexes) {
        if ($schemaContent -notmatch "CREATE INDEX.*$index") {
            throw "Index definition missing: $index"
        }
    }
}

Test-Step "Schema has backup triggers" {
    $requiredTriggers = @(
        "backup_provider_on_update",
        "backup_provider_on_insert",
        "backup_provider_on_delete"
    )

    foreach ($trigger in $requiredTriggers) {
        if ($schemaContent -notmatch "CREATE TRIGGER.*$trigger") {
            throw "Trigger definition missing: $trigger"
        }
    }
}

Test-Step "Schema has views for querying" {
    $requiredViews = @(
        "provider_status",
        "available_providers"
    )

    foreach ($view in $requiredViews) {
        if ($schemaContent -notmatch "CREATE VIEW.*$view") {
            throw "View definition missing: $view"
        }
    }
}

Write-Host ""

# ============================================================================
# Phase 3: Code Compilation Validation
# ============================================================================

Write-Host "Phase 3: Validating Code Compilation..." -ForegroundColor Cyan
Write-Host ""

Test-Step "Go module tidy" {
    Push-Location $projectRoot
    try {
        $output = go mod tidy 2>&1
        if ($LASTEXITCODE -ne 0) {
            throw "go mod tidy failed: $output"
        }
    }
    finally {
        Pop-Location
    }
}

Test-Step "Migration package compiles" {
    Push-Location (Join-Path $servicesDir "migration")
    try {
        $output = go build 2>&1
        if ($LASTEXITCODE -ne 0) {
            throw "Migration package compilation failed: $output"
        }
    }
    finally {
        Pop-Location
    }
}

Test-Step "Services package compiles" {
    Push-Location $servicesDir
    try {
        $output = go build 2>&1
        if ($LASTEXITCODE -ne 0) {
            throw "Services package compilation failed: $output"
        }
    }
    finally {
        Pop-Location
    }
}

Write-Host ""

# ============================================================================
# Phase 4: Unit Tests Execution
# ============================================================================

Write-Host "Phase 4: Running Unit Tests..." -ForegroundColor Cyan
Write-Host ""

Test-Step "Migration tests pass" {
    Push-Location (Join-Path $servicesDir "migration")
    try {
        $output = go test -v -timeout 30s 2>&1
        Write-Host $output -ForegroundColor Gray
        if ($LASTEXITCODE -ne 0) {
            throw "Migration tests failed"
        }
    }
    finally {
        Pop-Location
    }
}

Test-Step "Provider Service V2 tests pass" {
    Push-Location $servicesDir
    try {
        $output = go test -v -run "TestProviderServiceV2" -timeout 30s 2>&1
        Write-Host $output -ForegroundColor Gray
        if ($LASTEXITCODE -ne 0) {
            throw "ProviderServiceV2 tests failed"
        }
    }
    finally {
        Pop-Location
    }
}

Write-Host ""

# ============================================================================
# Phase 5: Backward Compatibility Validation
# ============================================================================

Write-Host "Phase 5: Validating Backward Compatibility..." -ForegroundColor Cyan
Write-Host ""

Test-Step "ProviderService (v1) interface compatibility" {
    $v1Path = Join-Path $servicesDir "providerservice.go"
    $v2Path = Join-Path $servicesDir "providerservice_v2.go"

    $v1Content = Get-Content $v1Path -Raw
    $v2Content = Get-Content $v2Path -Raw

    # Check that V2 has all V1 public methods
    $requiredMethods = @(
        "LoadProviders",
        "SaveProviders",
        "Start",
        "Stop"
    )

    foreach ($method in $requiredMethods) {
        if ($v2Content -notmatch "func.*\(ps \*ProviderServiceV2\) $method") {
            throw "Method missing in V2: $method"
        }
    }
}

Test-Step "Provider struct field compatibility" {
    $v1Path = Join-Path $servicesDir "providerservice.go"
    $v2Path = Join-Path $servicesDir "providerservice_v2.go"

    $v1Content = Get-Content $v1Path -Raw
    $v2Content = Get-Content $v2Path -Raw

    # Check that Provider struct is shared (not duplicated)
    $v1ProviderCount = ([regex]::Matches($v1Content, "type Provider struct")).Count
    $v2ProviderCount = ([regex]::Matches($v2Content, "type Provider struct")).Count

    if ($v2ProviderCount -gt 0) {
        throw "Provider struct should not be duplicated in V2 (should import from v1)"
    }
}

Write-Host ""

# ============================================================================
# Results Summary
# ============================================================================

Write-Host "===========================================" -ForegroundColor Cyan
Write-Host "  Validation Results" -ForegroundColor Cyan
Write-Host "===========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Total Tests: $script:TotalTests" -ForegroundColor White
Write-Host "Passed:      $script:TestsPassed" -ForegroundColor Green
Write-Host "Failed:      $script:TestsFailed" -ForegroundColor $(if ($script:TestsFailed -gt 0) { "Red" } else { "Green" })
Write-Host ""

if ($script:TestsFailed -eq 0) {
    Write-Host "✓ Phase 1 Migration Implementation: VALIDATED" -ForegroundColor Green
    Write-Host ""
    Write-Host "All validation checks passed. The migration is ready for integration." -ForegroundColor Green
    exit 0
}
else {
    Write-Host "✗ Phase 1 Migration Implementation: VALIDATION FAILED" -ForegroundColor Red
    Write-Host ""
    Write-Host "Some validation checks failed. Please review the errors above." -ForegroundColor Red
    exit 1
}
