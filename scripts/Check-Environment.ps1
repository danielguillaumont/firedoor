# Safe, read-only Windows PowerShell host inventory.
$ErrorActionPreference = 'Stop'
Write-Host 'FIREDOOR - v0.0 host inventory (read only)' -ForegroundColor Cyan
Write-Host 'No cluster is created and no configuration is applied.'
Write-Host ''

try {
    $os = Get-CimInstance Win32_OperatingSystem -ErrorAction Stop
    $computer = Get-CimInstance Win32_ComputerSystem -ErrorAction Stop
    $processor = Get-CimInstance Win32_Processor -ErrorAction Stop | Select-Object -First 1
    Write-Host ('OS:       ' + $os.Caption + ' ' + $os.Version)
    Write-Host ('Memory:   ' + [math]::Round($computer.TotalPhysicalMemory / 1GB, 1) + ' GiB')
    Write-Host ('CPU:      ' + $processor.Name + ' (' + $processor.NumberOfLogicalProcessors + ' logical processors)')
} catch {
    Write-Host ('OS detail unavailable: ' + $_.Exception.Message) -ForegroundColor Yellow
}
Write-Host ''

# No invocations of kubectl, docker, kind or terraform are made here.
$tools = @('git','go','terraform','docker','kind','kubectl','node','npm','ssh','gh')
foreach ($tool in $tools) {
    $found = Get-Command $tool -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $found) {
        Write-Host ('MISSING  ' + $tool) -ForegroundColor Yellow
    } else {
        Write-Host ('FOUND    ' + $tool + '  ' + $found.Source) -ForegroundColor Green
    }
}
Write-Host ''
Write-Host 'Next: run version commands manually from the chat instructions, then go mod tidy and foundation tests.'
