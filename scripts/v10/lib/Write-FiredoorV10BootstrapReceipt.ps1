# Firedoor 10C2C4 receipt helper: isolated local filesystem writes only.
# Dot-source after the bootstrap dry-run gate. No infrastructure operations.
function Write-Receipt([string]$File, [object]$Data) {
    if ([string]::IsNullOrWhiteSpace($File) -or -not [IO.Path]::IsPathRooted($File)) {
        throw 'Receipt path must be an absolute filesystem path. Stop.'
    }
    $full = [IO.Path]::GetFullPath($File)
    $directory = [IO.Path]::GetDirectoryName($full)
    if (-not [IO.Directory]::Exists($directory)) {
        throw 'Receipt parent directory does not exist. Stop.'
    }
    if (([IO.File]::GetAttributes($directory) -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw 'Linked receipt directory refused. Stop.'
    }
    $temp = $full + '.new'
    $backup = $full + '.previous'
    foreach ($reserved in @($temp, $backup)) {
        if ([IO.File]::Exists($reserved) -or [IO.Directory]::Exists($reserved)) {
            throw "Reserved receipt file already exists: $reserved. Preserve evidence. Stop."
        }
    }
    if ([IO.Directory]::Exists($full)) { throw 'Receipt path is a directory. Stop.' }
    if ([IO.File]::Exists($full) -and
        ([IO.File]::GetAttributes($full) -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw 'Linked receipt destination refused. Stop.'
    }

    $json = $Data | ConvertTo-Json -Depth 8 -ErrorAction Stop
    if ([string]::IsNullOrWhiteSpace($json)) { throw 'Empty receipt JSON refused. Stop.' }
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    [IO.File]::WriteAllText($temp, $json, $utf8)
    if ([IO.File]::Exists($full)) {
        # Windows PowerShell 5.1 / .NET Framework rejects a null backup path.
        # The explicit same-directory backup retains the original receipt.
        [IO.File]::Replace($temp, $full, $backup)
    } else {
        # New receipt: create only; no overwrite or historical promotion.
        [IO.File]::Move($temp, $full)
    }
}
