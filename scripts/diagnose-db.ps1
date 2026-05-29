# Database diagnostic script for OpenForge (Windows PowerShell)
# Run this script to diagnose database connection issues

Write-Host "=== OpenForge Database Diagnostics ===" -ForegroundColor Cyan
Write-Host "Timestamp: $(Get-Date)" -ForegroundColor Gray
Write-Host

# 1. Check PostgreSQL service status
Write-Host "1. PostgreSQL Service Status:" -ForegroundColor Yellow
try {
    $pgReady = & pg_isready -h localhost -p 5432 2>&1
    Write-Host "   $pgReady" -ForegroundColor Green
} catch {
    Write-Host "   pg_isready not found, checking with psql..." -ForegroundColor Yellow
    try {
        $result = & psql -h localhost -p 5432 -U openforge -d openforge -c "SELECT 1;" 2>&1
        Write-Host "   Connection successful" -ForegroundColor Green
    } catch {
        Write-Host "   Connection failed: $_" -ForegroundColor Red
    }
}
Write-Host

# 2. Check PostgreSQL configuration
Write-Host "2. PostgreSQL Configuration:" -ForegroundColor Yellow
try {
    $config = & psql -h localhost -p 5432 -U openforge -d openforge -c @"
SELECT name, setting, unit, context 
FROM pg_settings 
WHERE name IN (
    'max_connections',
    'shared_buffers',
    'work_mem',
    'maintenance_work_mem',
    'effective_cache_size',
    'random_page_cost',
    'checkpoint_completion_target',
    'wal_buffers',
    'default_statistics_target'
)
ORDER BY name;
"@ 2>&1
    Write-Host $config -ForegroundColor White
} catch {
    Write-Host "   Failed to get configuration: $_" -ForegroundColor Red
}
Write-Host

# 3. Check current connections
Write-Host "3. Current Database Connections:" -ForegroundColor Yellow
try {
    $connections = & psql -h localhost -p 5432 -U openforge -d openforge -c @"
SELECT 
    count(*) as total_connections,
    count(*) FILTER (WHERE state = 'active') as active,
    count(*) FILTER (WHERE state = 'idle') as idle,
    count(*) FILTER (WHERE state = 'idle in transaction') as idle_in_transaction,
    count(*) FILTER (WHERE state = 'fastpath function call') as fastpath
FROM pg_stat_activity
WHERE datname = 'openforge';
"@ 2>&1
    Write-Host $connections -ForegroundColor White
} catch {
    Write-Host "   Failed to get connections: $_" -ForegroundColor Red
}
Write-Host

# 4. Check connection details
Write-Host "4. Connection Details:" -ForegroundColor Yellow
try {
    $details = & psql -h localhost -p 5432 -U openforge -d openforge -c @"
SELECT 
    pid,
    usename,
    application_name,
    client_addr,
    backend_start,
    xact_start,
    query_start,
    state,
    LEFT(query, 100) as query_preview
FROM pg_stat_activity
WHERE datname = 'openforge'
ORDER BY backend_start DESC
LIMIT 20;
"@ 2>&1
    Write-Host $details -ForegroundColor White
} catch {
    Write-Host "   Failed to get details: $_" -ForegroundColor Red
}
Write-Host

# 5. Check for long-running queries
Write-Host "5. Long-running Queries (>30 seconds):" -ForegroundColor Yellow
try {
    $longQueries = & psql -h localhost -p 5432 -U openforge -d openforge -c @"
SELECT 
    pid,
    usename,
    NOW() - query_start as duration,
    state,
    LEFT(query, 200) as query_preview
FROM pg_stat_activity
WHERE datname = 'openforge'
    AND state = 'active'
    AND NOW() - query_start > INTERVAL '30 seconds'
ORDER BY duration DESC;
"@ 2>&1
    Write-Host $longQueries -ForegroundColor White
} catch {
    Write-Host "   Failed to get long queries: $_" -ForegroundColor Red
}
Write-Host

# 6. Check for idle in transaction connections
Write-Host "6. Idle in Transaction Connections:" -ForegroundColor Yellow
try {
    $idleInTx = & psql -h localhost -p 5432 -U openforge -d openforge -c @"
SELECT 
    pid,
    usename,
    NOW() - xact_start as xact_duration,
    NOW() - query_start as query_duration,
    state,
    LEFT(query, 200) as query_preview
FROM pg_stat_activity
WHERE datname = 'openforge'
    AND state = 'idle in transaction'
ORDER BY xact_duration DESC;
"@ 2>&1
    Write-Host $idleInTx -ForegroundColor White
} catch {
    Write-Host "   Failed to get idle transactions: $_" -ForegroundColor Red
}
Write-Host

# 7. Check database size and table sizes
Write-Host "7. Database and Table Sizes:" -ForegroundColor Yellow
try {
    $dbSize = & psql -h localhost -p 5432 -U openforge -d openforge -c "SELECT pg_size_pretty(pg_database_size('openforge')) as database_size;" 2>&1
    Write-Host "   Database size: $dbSize" -ForegroundColor White
    
    $tableSizes = & psql -h localhost -p 5432 -U openforge -d openforge -c @"
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
    pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)) as table_size,
    pg_size_pretty(pg_indexes_size(schemaname||'.'||tablename)) as index_size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
LIMIT 10;
"@ 2>&1
    Write-Host $tableSizes -ForegroundColor White
} catch {
    Write-Host "   Failed to get sizes: $_" -ForegroundColor Red
}
Write-Host

# 8. Check for locks
Write-Host "8. Current Locks:" -ForegroundColor Yellow
try {
    $locks = & psql -h localhost -p 5432 -U openforge -d openforge -c @"
SELECT 
    l.pid,
    l.mode,
    l.granted,
    a.usename,
    a.state,
    LEFT(a.query, 100) as query_preview
FROM pg_locks l
JOIN pg_stat_activity a ON l.pid = a.pid
WHERE l.relation IS NOT NULL
ORDER BY l.pid;
"@ 2>&1
    Write-Host $locks -ForegroundColor White
} catch {
    Write-Host "   Failed to get locks: $_" -ForegroundColor Red
}
Write-Host

# 9. Check system resources
Write-Host "9. System Resources:" -ForegroundColor Yellow
Write-Host "Memory:" -ForegroundColor White
try {
    $memory = Get-CimInstance -ClassName Win32_OperatingSystem
    $totalMemory = [math]::Round($memory.TotalVisibleMemorySize / 1MB, 2)
    $freeMemory = [math]::Round($memory.FreePhysicalMemory / 1MB, 2)
    $usedMemory = $totalMemory - $freeMemory
    Write-Host "   Total: ${totalMemory}GB, Used: ${usedMemory}GB, Free: ${freeMemory}GB" -ForegroundColor White
} catch {
    Write-Host "   Failed to get memory info: $_" -ForegroundColor Red
}

Write-Host "Disk Usage:" -ForegroundColor White
try {
    $disk = Get-CimInstance -ClassName Win32_LogicalDisk -Filter "DeviceID='C:'"
    $totalDisk = [math]::Round($disk.Size / 1GB, 2)
    $freeDisk = [math]::Round($disk.FreeSpace / 1GB, 2)
    $usedDisk = $totalDisk - $freeDisk
    Write-Host "   C: Total: ${totalDisk}GB, Used: ${usedDisk}GB, Free: ${freeDisk}GB" -ForegroundColor White
} catch {
    Write-Host "   Failed to get disk info: $_" -ForegroundColor Red
}

Write-Host "CPU Load:" -ForegroundColor White
try {
    $cpu = Get-CimInstance -ClassName Win32_Processor
    $load = $cpu.LoadPercentage
    Write-Host "   CPU Load: ${load}%" -ForegroundColor White
} catch {
    Write-Host "   Failed to get CPU info: $_" -ForegroundColor Red
}
Write-Host

Write-Host "=== Diagnostics Complete ===" -ForegroundColor Cyan
Write-Host "If you see connection issues, consider:" -ForegroundColor Yellow
Write-Host "1. Increasing max_connections in postgresql.conf" -ForegroundColor White
Write-Host "2. Adding connection timeout to database queries" -ForegroundColor White
Write-Host "3. Checking for connection leaks in application code" -ForegroundColor White
Write-Host "4. Monitoring for long-running queries" -ForegroundColor White
Write-Host "5. Checking PostgreSQL logs for errors" -ForegroundColor White
