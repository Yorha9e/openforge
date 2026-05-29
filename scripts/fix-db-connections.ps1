# Quick fix script for database connection issues
# Run this script to apply immediate fixes

Write-Host "=== OpenForge Database Connection Quick Fix ===" -ForegroundColor Cyan
Write-Host

# 1. Check if PostgreSQL is running
Write-Host "1. Checking PostgreSQL service..." -ForegroundColor Yellow
try {
    $pgService = Get-Service -Name "postgresql*" -ErrorAction SilentlyContinue
    if ($pgService) {
        Write-Host "   PostgreSQL service found: $($pgService.Name) - Status: $($pgService.Status)" -ForegroundColor Green
        if ($pgService.Status -ne "Running") {
            Write-Host "   Starting PostgreSQL service..." -ForegroundColor Yellow
            Start-Service $pgService.Name
            Write-Host "   PostgreSQL service started." -ForegroundColor Green
        }
    } else {
        Write-Host "   PostgreSQL service not found. Checking with pg_isready..." -ForegroundColor Yellow
        $pgReady = & pg_isready -h localhost -p 5432 2>&1
        Write-Host "   $pgReady" -ForegroundColor White
    }
} catch {
    Write-Host "   Error checking PostgreSQL: $_" -ForegroundColor Red
}
Write-Host

# 2. Increase PostgreSQL max_connections (if possible)
Write-Host "2. Checking PostgreSQL configuration..." -ForegroundColor Yellow
try {
    $configPath = ""
    $possiblePaths = @(
        "C:\Program Files\PostgreSQL\*\data\postgresql.conf",
        "C:\PostgreSQL\*\data\postgresql.conf",
        "$env:APPDATA\PostgreSQL\*\data\postgresql.conf"
    )
    
    foreach ($path in $possiblePaths) {
        $found = Get-ChildItem -Path $path -ErrorAction SilentlyContinue
        if ($found) {
            $configPath = $found.FullName
            break
        }
    }
    
    if ($configPath) {
        Write-Host "   Found PostgreSQL config: $configPath" -ForegroundColor Green
        
        # Read current max_connections
        $configContent = Get-Content $configPath -Raw
        if ($configContent -match "max_connections\s*=\s*(\d+)") {
            $currentMax = $matches[1]
            Write-Host "   Current max_connections: $currentMax" -ForegroundColor White
            
            if ([int]$currentMax -lt 200) {
                Write-Host "   Increasing max_connections to 200..." -ForegroundColor Yellow
                $configContent = $configContent -replace "max_connections\s*=\s*\d+", "max_connections = 200"
                Set-Content -Path $configPath -Value $configContent -Encoding UTF8
                Write-Host "   max_connections increased to 200." -ForegroundColor Green
                Write-Host "   NOTE: PostgreSQL service needs to be restarted for changes to take effect." -ForegroundColor Yellow
            } else {
                Write-Host "   max_connections is already sufficient." -ForegroundColor Green
            }
        }
    } else {
        Write-Host "   PostgreSQL config file not found." -ForegroundColor Yellow
        Write-Host "   Please manually increase max_connections in postgresql.conf" -ForegroundColor Yellow
    }
} catch {
    Write-Host "   Error checking config: $_" -ForegroundColor Red
}
Write-Host

# 3. Kill any long-running queries
Write-Host "3. Checking for long-running queries..." -ForegroundColor Yellow
try {
    $longQueries = & psql -h localhost -p 5432 -U openforge -d openforge -c @"
SELECT pid, NOW() - query_start as duration, LEFT(query, 100) as query
FROM pg_stat_activity
WHERE datname = 'openforge'
    AND state = 'active'
    AND NOW() - query_start > INTERVAL '60 seconds';
"@ 2>&1
    
    if ($longQueries -match "\d+") {
        Write-Host "   Found long-running queries:" -ForegroundColor Yellow
        Write-Host $longQueries -ForegroundColor White
        
        $pids = [regex]::Matches($longQueries, "\b\d{4,}\b")
        foreach ($pid in $pids) {
            Write-Host "   Terminating query with PID: $($pid.Value)" -ForegroundColor Yellow
            & psql -h localhost -p 5432 -U openforge -d openforge -c "SELECT pg_terminate_backend($($pid.Value));" 2>&1 | Out-Null
        }
        Write-Host "   Long-running queries terminated." -ForegroundColor Green
    } else {
        Write-Host "   No long-running queries found." -ForegroundColor Green
    }
} catch {
    Write-Host "   Error checking queries: $_" -ForegroundColor Red
}
Write-Host

# 4. Kill idle in transaction connections
Write-Host "4. Checking for idle in transaction connections..." -ForegroundColor Yellow
try {
    $idleInTx = & psql -h localhost -p 5432 -U openforge -d openforge -c @"
SELECT pid, NOW() - xact_start as duration
FROM pg_stat_activity
WHERE datname = 'openforge'
    AND state = 'idle in transaction'
    AND NOW() - xact_start > INTERVAL '5 minutes';
"@ 2>&1
    
    if ($idleInTx -match "\d+") {
        Write-Host "   Found idle in transaction connections:" -ForegroundColor Yellow
        Write-Host $idleInTx -ForegroundColor White
        
        $pids = [regex]::Matches($idleInTx, "\b\d{4,}\b")
        foreach ($pid in $pids) {
            Write-Host "   Terminating connection with PID: $($pid.Value)" -ForegroundColor Yellow
            & psql -h localhost -p 5432 -U openforge -d openforge -c "SELECT pg_terminate_backend($($pid.Value));" 2>&1 | Out-Null
        }
        Write-Host "   Idle connections terminated." -ForegroundColor Green
    } else {
        Write-Host "   No idle in transaction connections found." -ForegroundColor Green
    }
} catch {
    Write-Host "   Error checking connections: $_" -ForegroundColor Red
}
Write-Host

# 5. Restart application if running
Write-Host "5. Checking for running OpenForge application..." -ForegroundColor Yellow
try {
    $appProcesses = Get-Process -Name "openforge*" -ErrorAction SilentlyContinue
    if ($appProcesses) {
        Write-Host "   Found OpenForge processes:" -ForegroundColor Yellow
        $appProcesses | Format-Table Id, ProcessName, StartTime -AutoSize
        
        $restart = Read-Host "   Do you want to restart the application? (y/n)"
        if ($restart -eq "y") {
            Write-Host "   Stopping OpenForge processes..." -ForegroundColor Yellow
            $appProcesses | Stop-Process -Force
            Start-Sleep -Seconds 2
            
            Write-Host "   Starting OpenForge application..." -ForegroundColor Yellow
            # Note: This assumes the application is started with 'go run' or similar
            # Adjust the command based on your setup
            Write-Host "   Please manually restart the application." -ForegroundColor Yellow
        }
    } else {
        Write-Host "   No OpenForge processes found." -ForegroundColor Green
    }
} catch {
    Write-Host "   Error checking processes: $_" -ForegroundColor Red
}
Write-Host

# 6. Create a test connection
Write-Host "6. Testing database connection..." -ForegroundColor Yellow
try {
    $testResult = & psql -h localhost -p 5432 -U openforge -d openforge -c "SELECT 1 as test;" 2>&1
    if ($testResult -match "1") {
        Write-Host "   Database connection successful." -ForegroundColor Green
    } else {
        Write-Host "   Database connection failed: $testResult" -ForegroundColor Red
    }
} catch {
    Write-Host "   Database connection failed: $_" -ForegroundColor Red
}
Write-Host

Write-Host "=== Quick Fix Complete ===" -ForegroundColor Cyan
Write-Host
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. If you increased max_connections, restart PostgreSQL service." -ForegroundColor White
Write-Host "2. Restart the OpenForge application." -ForegroundColor White
Write-Host "3. Monitor the health check endpoint: http://localhost:8080/api/health/db" -ForegroundColor White
Write-Host "4. If issues persist, run the full diagnostic script: .\scripts\diagnose-db.ps1" -ForegroundColor White
Write-Host "5. Check the troubleshooting guide: docs\database-connection-troubleshooting.md" -ForegroundColor White
