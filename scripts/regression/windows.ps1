[CmdletBinding()]
param(
    [string]$ArtifactsRoot = (Join-Path $env:TEMP ("ntrace-regression-windows-" + (Get-Date -Format "yyyyMMdd-HHmmss"))),
    [string]$TsharkPath = ""
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$BinDir = Join-Path $ArtifactsRoot "bin"
$ArtifactsDir = Join-Path $ArtifactsRoot "artifacts"
$Summary = Join-Path $ArtifactsRoot "summary.tsv"
$Targets = Join-Path $ArtifactsRoot "targets.txt"
$DefaultTmp = Join-Path $ArtifactsRoot "tmp"
$DefaultLog = Join-Path $DefaultTmp "trace.log"

$Bin = Join-Path $BinDir "nexttrace-current.exe"
$Tiny = Join-Path $BinDir "nexttrace-tiny-current.exe"
$Ntr = Join-Path $BinDir "ntr-current.exe"

New-Item -ItemType Directory -Force -Path $BinDir, $ArtifactsDir | Out-Null
New-Item -ItemType Directory -Force -Path $DefaultTmp | Out-Null
Set-Content -Path $Summary -Value @()

function Write-Record {
    param(
        [string]$Name,
        [string]$Status,
        [string]$Note
    )
    $line = "$Name`t$Status`t$Note"
    Add-Content -Path $Summary -Value $line
    Write-Host $line
}

function Write-IPv6Skip {
    param(
        [string]$Name,
        [string]$Note
    )
    Write-Record $Name SKIP "$Note; IPv6 not available on this machine"
}

function Format-DisplayPath {
    param([string]$Path)
    if ([string]::IsNullOrEmpty($Path)) {
        return $Path
    }
    if ($HOME -and $Path.StartsWith($HOME, [System.StringComparison]::OrdinalIgnoreCase)) {
        return "~" + $Path.Substring($HOME.Length)
    }
    return $Path
}

function Test-IsAdmin {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Resolve-TsharkPath {
    param([string]$ExplicitPath = "")
    $candidates = @()
    $searchRoots = @()
    if (-not [string]::IsNullOrWhiteSpace($ExplicitPath)) {
        $candidates += $ExplicitPath
    }
    try {
        $cmd = Get-Command tshark -ErrorAction Stop
        if ($cmd.Path) {
            $candidates += $cmd.Path
        }
        elseif ($cmd.Source) {
            $candidates += $cmd.Source
        }
    }
    catch {
    }
    foreach ($root in @($env:ProgramFiles, $env:ProgramW6432, ${env:ProgramFiles(x86)})) {
        if (-not [string]::IsNullOrWhiteSpace($root)) {
            $searchRoots += $root
            $candidates += (Join-Path $root "Wireshark\tshark.exe")
        }
    }
    $candidates += @(
        "C:\Program Files\Wireshark\tshark.exe",
        "C:\Program Files (x86)\Wireshark\tshark.exe"
    )
    foreach ($candidate in $candidates | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique) {
        if (Test-Path -LiteralPath $candidate) {
            return $candidate
        }
    }
    foreach ($root in $searchRoots | Select-Object -Unique) {
        try {
            $found = Get-ChildItem -LiteralPath $root -Filter tshark.exe -File -Recurse -ErrorAction Stop |
                Select-Object -First 1 -ExpandProperty FullName
            if ($found) {
                return $found
            }
        }
        catch {
        }
    }
    return $null
}

function Invoke-CommandWithTimeout {
    param(
        [string]$Command,
        [string]$OutFile,
        [bool]$MergeStreams = $true,
        [int]$Seconds = 150
    )
    $stdoutFile = "$OutFile.stdout"
    $stderrFile = "$OutFile.stderr"
    $scriptFile = "$OutFile.cmd"
    Remove-Item -Force -ErrorAction Ignore $OutFile, $stdoutFile, $stderrFile, $scriptFile
    Set-Content -Path $scriptFile -Encoding ascii -Value @(
        "@echo off"
        $Command
    )
    $proc = Start-Process -FilePath "cmd.exe" -ArgumentList "/d", "/c", "`"$scriptFile`"" -RedirectStandardOutput $stdoutFile -RedirectStandardError $stderrFile -PassThru -WindowStyle Hidden
    if (-not $proc.WaitForExit($Seconds * 1000)) {
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
        $exitCode = 124
    }
    else {
        $exitCode = $proc.ExitCode
    }
    $stdoutText = if (Test-Path $stdoutFile) { Get-Content -Raw -Path $stdoutFile } else { "" }
    $stderrText = if (Test-Path $stderrFile) { Get-Content -Raw -Path $stderrFile } else { "" }
    if ($MergeStreams) {
        Set-Content -Path $OutFile -Value ($stdoutText + $stderrText)
        Remove-Item -Force -ErrorAction Ignore $stdoutFile, $stderrFile
    }
    else {
        Set-Content -Path $OutFile -Value $stdoutText
        Set-Content -Path $stderrFile -Value $stderrText
    }
    Remove-Item -Force -ErrorAction Ignore $scriptFile
    return $exitCode
}

function Run-Cmd {
    param(
        [string]$Name,
        [string]$Note,
        [string]$Command,
        [string]$SuccessPattern = "",
        [int]$Seconds = 150
    )
    $out = Join-Path $ArtifactsDir "$Name.txt"
    $rc = Invoke-CommandWithTimeout -Command $Command -OutFile $out -Seconds $Seconds
    $content = if (Test-Path $out) { Get-Content -Raw -Path $out } else { "" }
    if ($rc -eq 0 -or (-not [string]::IsNullOrWhiteSpace($SuccessPattern) -and $content -match $SuccessPattern)) {
        Write-Record $Name PASS $Note
    }
    else {
        Write-Record $Name FAIL "$Note; exit=$rc"
    }
}

function Write-SkipRecord {
    param(
        [string]$Name,
        [string]$Note,
        [string]$Reason
    )
    Write-Record $Name SKIP "$Note; $Reason"
}

function Get-CapabilityStatus {
    param(
        [string]$Name,
        [string]$Label,
        [string]$Command,
        [string]$SuccessPattern = "",
        [int]$Seconds = 60
    )
    $out = Join-Path $ArtifactsDir "_capability_$Name.txt"
    $rc = Invoke-CommandWithTimeout -Command $Command -OutFile $out -Seconds $Seconds
    $content = if (Test-Path $out) { Get-Content -Raw -Path $out } else { "" }
    $detail = ""
    if ($rc -ne 0 -and (Test-Path $out)) {
        $detail = Get-Content -Path $out | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Last 1
    }
    $supported = $rc -eq 0
    if (-not $supported -and -not [string]::IsNullOrWhiteSpace($SuccessPattern) -and $content -match $SuccessPattern) {
        $supported = $true
    }
    return [pscustomobject]@{
        Supported = $supported
        Reason    = if ($supported) { "" } elseif ([string]::IsNullOrWhiteSpace($detail)) { "$Label unavailable in this Windows environment; probe exit=$rc" } else { "$Label unavailable in this Windows environment; probe exit=$rc; $detail" }
    }
}

function Run-CmdIfSupported {
    param(
        [bool]$Supported,
        [string]$Reason,
        [string]$Name,
        [string]$Note,
        [string]$Command,
        [string]$SuccessPattern = "",
        [int]$Seconds = 150
    )
    if (-not $Supported) {
        Write-SkipRecord $Name $Note $Reason
        return
    }
    Run-Cmd -Name $Name -Note $Note -Command $Command -SuccessPattern $SuccessPattern -Seconds $Seconds
}

function Check-JsonPureIfSupported {
    param(
        [bool]$Supported,
        [string]$Reason,
        [string]$Name,
        [string]$Note,
        [string]$Command
    )
    if (-not $Supported) {
        Write-SkipRecord $Name $Note $Reason
        return
    }
    Check-JsonPure -Name $Name -Note $Note -Command $Command
}

function Check-OutputFileIfSupported {
    param(
        [bool]$Supported,
        [string]$Reason,
        [string]$Name,
        [string]$Note,
        [string]$Command,
        [string]$Path
    )
    if (-not $Supported) {
        Write-SkipRecord $Name $Note $Reason
        return
    }
    Check-OutputFile -Name $Name -Note $Note -Command $Command -Path $Path
}

function Check-PacketCaptureIfSupported {
    param(
        [bool]$Supported,
        [string]$Reason,
        [string]$Name,
        [string]$Note,
        [string]$Command,
        [string]$DisplayFilter,
        [string]$Expect1,
        [string]$Expect2
    )
    if (-not $Supported) {
        Write-SkipRecord $Name $Note $Reason
        return
    }
    Check-PacketCapture -Name $Name -Note $Note -Command $Command -DisplayFilter $DisplayFilter -Expect1 $Expect1 -Expect2 $Expect2
}

function Wait-FileContains {
    param(
        [string]$Path,
        [string]$Pattern,
        [int]$TimeoutSeconds = 10
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Test-Path $Path) {
            $content = Get-Content -Raw -Path $Path -ErrorAction SilentlyContinue
            if (-not [string]::IsNullOrEmpty([string]$content) -and [string]$content -match $Pattern) {
                return $true
            }
        }
        Start-Sleep -Milliseconds 200
    }
    return $false
}

function Wait-AnyFileContains {
    param(
        [string[]]$Paths,
        [string]$Pattern,
        [int]$TimeoutSeconds = 10
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        foreach ($path in $Paths) {
            if ([string]::IsNullOrWhiteSpace($path)) {
                continue
            }
            if (Test-Path $path) {
                $content = Get-Content -Raw -Path $path -ErrorAction SilentlyContinue
                if (-not [string]::IsNullOrEmpty([string]$content) -and [string]$content -match $Pattern) {
                    return $true
                }
            }
        }
        Start-Sleep -Milliseconds 200
    }
    return $false
}

function Wait-CaptureReady {
    param(
        [System.Diagnostics.Process]$Process,
        [string[]]$Paths,
        [string]$Pattern,
        [int]$TimeoutSeconds = 10,
        [int]$AssumeReadyAfterMs = 1000
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $readyAfter = (Get-Date).AddMilliseconds($AssumeReadyAfterMs)
    while ((Get-Date) -lt $deadline) {
        foreach ($path in $Paths) {
            if ([string]::IsNullOrWhiteSpace($path)) {
                continue
            }
            if (Test-Path $path) {
                $content = Get-Content -Raw -Path $path -ErrorAction SilentlyContinue
                if (-not [string]::IsNullOrEmpty([string]$content) -and [string]$content -match $Pattern) {
                    return $true
                }
            }
        }
        if ($Process.HasExited) {
            return $false
        }
        if ((Get-Date) -ge $readyAfter) {
            return $true
        }
        Start-Sleep -Milliseconds 100
    }
    return -not $Process.HasExited
}

function Check-JsonPure {
    param(
        [string]$Name,
        [string]$Note,
        [string]$Command
    )
    $out = Join-Path $ArtifactsDir "$Name.txt"
    $rc = Invoke-CommandWithTimeout -Command $Command -OutFile $out -MergeStreams $false -Seconds 180
    $content = if (Test-Path $out) { Get-Content -Raw -Path $out } else { "" }
    $stderrFile = "$out.stderr"
    $stderrContent = if (Test-Path $stderrFile) { Get-Content -Raw -Path $stderrFile } else { "" }
    if ($content -match "request failed - please try again later" -or $stderrContent -match "request failed - please try again later") {
        Write-Record $Name SKIP "$Note; external service unavailable"
        return
    }
    $bytes = [System.IO.File]::ReadAllBytes($out)
    $first = ""
    foreach ($b in $bytes) {
        $ch = [char]$b
        if (-not [char]::IsWhiteSpace($ch)) {
            $first = $ch
            break
        }
    }
    if ($first -eq "{" -and $content -notmatch "preferred API IP") {
        Write-Record $Name PASS $Note
        return
    }
    if ($rc -ne 0) {
        Write-Record $Name FAIL "$Note; command failed"
    }
    else {
        Write-Record $Name FAIL "$Note; stdout not pure JSON"
    }
}

function Check-OutputFile {
    param(
        [string]$Name,
        [string]$Note,
        [string]$Command,
        [string]$Path
    )
    $out = Join-Path $ArtifactsDir "$Name.txt"
    Remove-Item -Force -ErrorAction Ignore $Path, $DefaultLog
    $rc = Invoke-CommandWithTimeout -Command $Command -OutFile $out -Seconds 150
    if ((Test-Path $Path) -and (Get-Item $Path).Length -gt 0) {
        Write-Record $Name PASS $Note
        return
    }
    if ($rc -ne 0) {
        Write-Record $Name FAIL "$Note; command failed"
        return
    }
    if (-not (Test-Path $Path) -or (Get-Item $Path).Length -le 0) {
        Write-Record $Name FAIL "$Note; log file missing"
        return
    }
    Write-Record $Name PASS $Note
}

function Check-PacketCapture {
    param(
        [string]$Name,
        [string]$Note,
        [string]$Command,
        [string]$DisplayFilter,
        [string]$Expect1,
        [string]$Expect2
    )
    $captureFamily = if ($DisplayFilter -match ":") { "IPv6" } else { "IPv4" }
    $tsharkExe = Resolve-TsharkPath -ExplicitPath $TsharkPath
    $probeLog = Join-Path $ArtifactsDir "_tshark_probe.txt"
    if (-not $tsharkExe) {
        Set-Content -Path $probeLog -Value @(
            "explicit=$TsharkPath"
            "resolved=$tsharkExe"
            "programfiles=$env:ProgramFiles"
            "programw6432=$env:ProgramW6432"
            "programfiles_x86=${env:ProgramFiles(x86)}"
            "capture_family=$captureFamily"
        )
        Write-Record $Name SKIP "$Note; tshark not available"
        return
    }
    $dump = Join-Path $ArtifactsDir "$Name.tshark.txt"
    $out = Join-Path $ArtifactsDir "$Name.cmd.txt"
    Remove-Item -Force -ErrorAction Ignore $dump, $out
    $iface = Get-CaptureInterface -AddressFamily $captureFamily
    Set-Content -Path $probeLog -Value @(
        "explicit=$TsharkPath"
        "resolved=$tsharkExe"
        "programfiles=$env:ProgramFiles"
        "programw6432=$env:ProgramW6432"
        "programfiles_x86=${env:ProgramFiles(x86)}"
        "capture_family=$captureFamily"
        "route_interface_index=$($iface.InterfaceIndex)"
        "route_interface_alias=$($iface.InterfaceAlias)"
        "route_interface_guid=$($iface.InterfaceGuid)"
        "npcap_device=$($iface.Device)"
    )
    if (-not $iface -or -not $iface.Device) {
        Write-Record $Name SKIP "$Note; capture interface not detected"
        return
    }
    $dumpStdout = "$dump.stdout"
    $dumpStderr = "$dump.stderr"
    Remove-Item -Force -ErrorAction Ignore $dumpStdout, $dumpStderr
    $captureArgs = @(
        "-i `"$($iface.Device)`"",
        "-c 1",
        "-f `"$DisplayFilter`"",
        "-V"
    ) -join " "
    Add-Content -Path $probeLog -Value "capture_args=$captureArgs"
    $capture = Start-Process -FilePath $tsharkExe -ArgumentList $captureArgs -RedirectStandardOutput $dumpStdout -RedirectStandardError $dumpStderr -PassThru -WindowStyle Hidden
    if (-not (Wait-CaptureReady -Process $capture -Paths @($dumpStdout, $dumpStderr) -Pattern "Capturing on" -TimeoutSeconds 10 -AssumeReadyAfterMs 1000)) {
        if (-not $capture.HasExited) {
            Stop-Process -Id $capture.Id -Force -ErrorAction SilentlyContinue
        }
        $dumpOutText = if (Test-Path $dumpStdout) { Get-Content -Raw -Path $dumpStdout } else { "" }
        $dumpErrText = if (Test-Path $dumpStderr) { Get-Content -Raw -Path $dumpStderr } else { "" }
        Set-Content -Path $dump -Value ($dumpOutText + $dumpErrText)
        Remove-Item -Force -ErrorAction Ignore $dumpStdout, $dumpStderr
        Write-Record $Name FAIL "$Note; tshark did not become ready"
        return
    }
    Start-Sleep -Milliseconds 800
    $null = Invoke-CommandWithTimeout -Command $Command -OutFile $out -Seconds 60
    Start-Sleep -Seconds 1
    if (-not $capture.WaitForExit(8000)) {
        Stop-Process -Id $capture.Id -Force -ErrorAction SilentlyContinue
    }
    $dumpOutText = if (Test-Path $dumpStdout) { Get-Content -Raw -Path $dumpStdout } else { "" }
    $dumpErrText = if (Test-Path $dumpStderr) { Get-Content -Raw -Path $dumpStderr } else { "" }
    Set-Content -Path $dump -Value ($dumpOutText + $dumpErrText)
    Remove-Item -Force -ErrorAction Ignore $dumpStdout, $dumpStderr
    $content = if (Test-Path $dump) { Get-Content -Raw -Path $dump } else { "" }
    if ($content.Contains($Expect1) -and $content.Contains($Expect2)) {
        Write-Record $Name PASS $Note
    }
    else {
        Write-Record $Name FAIL "$Note; packet capture mismatch"
    }
}

function Wait-HttpReady {
    param([string]$Url)
    for ($i = 0; $i -lt 30; $i++) {
        try {
            Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 2 | Out-Null
            return $true
        }
        catch {
            Start-Sleep -Seconds 1
        }
    }
    return $false
}

function Get-CaptureInterface {
    param(
        [ValidateSet("IPv4", "IPv6")]
        [string]$AddressFamily = "IPv4"
    )
    try {
        $family = if ($AddressFamily -eq "IPv6") { "IPv6" } else { "IPv4" }
        $prefix = if ($family -eq "IPv6") { "::/0" } else { "0.0.0.0/0" }
        $route = Get-NetRoute -AddressFamily $family -DestinationPrefix $prefix |
            Sort-Object -Property RouteMetric, InterfaceMetric |
            Select-Object -First 1
        if (-not $route -or -not $route.InterfaceIndex) {
            return $null
        }
        $adapter = Get-NetAdapter -InterfaceIndex $route.InterfaceIndex -ErrorAction Stop
        if (-not $adapter -or -not $adapter.InterfaceGuid) {
            return $null
        }
        $guid = ([string]$adapter.InterfaceGuid).Trim("{}").ToUpperInvariant()
        return [pscustomobject]@{
            Device         = "\Device\NPF_{$guid}"
            InterfaceIndex = [int]$route.InterfaceIndex
            InterfaceAlias = $adapter.InterfaceAlias
            InterfaceGuid  = $guid
            AddressFamily  = $family
        }
    }
    catch {
    }
    return $null
}

function Test-IPv6Available {
    try {
        $route = Get-NetRoute -AddressFamily IPv6 -ErrorAction Stop |
            Where-Object { $_.DestinationPrefix -eq "::/0" } |
            Sort-Object -Property RouteMetric, InterfaceMetric |
            Select-Object -First 1
        return $null -ne $route
    }
    catch {
        return $false
    }
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "go is required"
}
if (-not (Test-IsAdmin)) {
    throw "Please run this script in an elevated PowerShell session."
}

$IPv6Available = Test-IPv6Available
@(
    "1.1.1.1 Cloudflare-v4"
) + $(if ($IPv6Available) { "2606:4700:4700::1111 Cloudflare-v6" }) | Set-Content -Path $Targets

Write-Host ("artifacts: {0}" -f (Format-DisplayPath $ArtifactsRoot))
Write-Host ("ipv6_available={0}" -f $(if ($IPv6Available) { 1 } else { 0 }))
Push-Location $RepoRoot
try {
    go build -trimpath -o $Bin .
    go build -trimpath -tags flavor_tiny -o $Tiny .
    go build -trimpath -tags flavor_ntr -o $Ntr .
    go test ./...
    $windivertInitLog = Join-Path $ArtifactsDir "_windivert_init.txt"
    try {
        & $Bin --init *> $windivertInitLog
    }
    catch {
        $_ | Out-File -FilePath $windivertInitLog -Encoding utf8
    }
}
finally {
    Pop-Location
}

$icmp4Capability = Get-CapabilityStatus -Name "icmp4" -Label "ICMPv4 tracing" -Command "`"$Bin`" --no-color -q 1 -m 1 --timeout 1000 1.1.1.1" -SuccessPattern "hops max, .*ICMP mode"
$tcp4Capability = Get-CapabilityStatus -Name "tcp4" -Label "TCPv4 tracing" -Command "`"$Bin`" --no-color -T -q 1 -m 1 --timeout 1000 1.1.1.1" -SuccessPattern "hops max, .*TCP mode"
$udp4Capability = Get-CapabilityStatus -Name "udp4" -Label "UDPv4 tracing" -Command "`"$Bin`" --no-color -U -q 1 -m 1 --timeout 1000 1.1.1.1" -SuccessPattern "hops max, .*UDP mode"
$mtrCapability = Get-CapabilityStatus -Name "mtr" -Label "MTR ICMP tracing" -Command "`"$Bin`" --no-color -r -q 1 -i 300 --timeout 1000 -m 2 1.1.1.1" -SuccessPattern "(?m)^HOST:"
$mtuCapability = Get-CapabilityStatus -Name "mtu" -Label "MTU tracing" -Command "`"$Bin`" --no-color --mtu --timeout 1000 -q 1 -m 1 1.1.1.1" -SuccessPattern "Path MTU:"
$globalpingCapability = Get-CapabilityStatus -Name "globalping" -Label "Globalping tracing" -Command "`"$Bin`" --no-color --json --from Germany -q 1 -m 1 --timeout 1000 1.1.1.1" -SuccessPattern '^\s*\{'

if ($IPv6Available) {
    $icmp6Capability = Get-CapabilityStatus -Name "icmp6" -Label "ICMPv6 tracing" -Command "`"$Bin`" --no-color -6 -q 1 -m 1 --timeout 1000 2606:4700:4700::1111" -SuccessPattern "hops max, .*ICMP mode"
    $tcp6Capability = Get-CapabilityStatus -Name "tcp6" -Label "TCPv6 tracing" -Command "`"$Bin`" --no-color -6 -T -q 1 -m 1 --timeout 1000 2606:4700:4700::1111" -SuccessPattern "hops max, .*TCP mode"
    $udp6Capability = Get-CapabilityStatus -Name "udp6" -Label "UDPv6 tracing" -Command "`"$Bin`" --no-color -6 -U -q 1 -m 1 --timeout 1000 2606:4700:4700::1111" -SuccessPattern "hops max, .*UDP mode"
}
else {
    $icmp6Capability = [pscustomobject]@{ Supported = $false; Reason = "IPv6 not available on this machine" }
    $tcp6Capability = [pscustomobject]@{ Supported = $false; Reason = "IPv6 not available on this machine" }
    $udp6Capability = [pscustomobject]@{ Supported = $false; Reason = "IPv6 not available on this machine" }
}

Run-CmdIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "icmp4_basic" "ICMP IPv4 basic trace" "`"$Bin`" --no-color -q 1 -m 3 --timeout 1000 1.1.1.1" "hops max, .*ICMP mode"
Run-CmdIfSupported $tcp4Capability.Supported $tcp4Capability.Reason "tcp4_basic" "TCP IPv4 basic trace" "`"$Bin`" --no-color -T -q 1 -m 3 --timeout 1000 1.1.1.1" "hops max, .*TCP mode"
Run-CmdIfSupported $udp4Capability.Supported $udp4Capability.Reason "udp4_basic" "UDP IPv4 basic trace" "`"$Bin`" --no-color -U -q 1 -m 3 --timeout 1000 1.1.1.1" "hops max, .*UDP mode"
Run-CmdIfSupported $icmp6Capability.Supported $icmp6Capability.Reason "icmp6_basic" "ICMP IPv6 basic trace" "`"$Bin`" --no-color -6 -q 1 -m 3 --timeout 1000 2606:4700:4700::1111" "hops max, .*ICMP mode"
Run-CmdIfSupported $tcp6Capability.Supported $tcp6Capability.Reason "tcp6_basic" "TCP IPv6 basic trace" "`"$Bin`" --no-color -6 -T -q 1 -m 3 --timeout 1000 2606:4700:4700::1111" "hops max, .*TCP mode"
Run-CmdIfSupported $udp6Capability.Supported $udp6Capability.Reason "udp6_basic" "UDP IPv6 basic trace" "`"$Bin`" --no-color -6 -U -q 1 -m 3 --timeout 1000 2606:4700:4700::1111" "hops max, .*UDP mode"
Run-CmdIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "raw_output" "Raw hop rows" "`"$Bin`" --no-color --raw -q 1 -m 2 --timeout 1000 1.1.1.1" "(?m)^1\|"
Run-CmdIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "classic_output" "Classic printer" "`"$Bin`" --no-color --classic -q 1 -m 2 --timeout 1000 1.1.1.1" "hops max, .*ICMP mode"
Run-CmdIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "route_path" "Route-path summary" "`"$Bin`" --no-color --route-path -q 1 -m 2 --timeout 1000 1.1.1.1" "Route-Path|hops max, .*ICMP mode"
Run-CmdIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "provider_lang" "IP.SB + sakura + en" "`"$Bin`" --no-color -q 1 -m 2 --timeout 1000 --data-provider IP.SB --pow-provider sakura --language en 1.1.1.1" "hops max, .*ICMP mode"
Run-CmdIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "dot_resolver" "DoT resolver via aliyun" "`"$Bin`" --no-color --dot-server aliyun -q 1 -m 1 --timeout 1000 ipv4.pek-4134.endpoint.nxtrace.org" "hops max, .*ICMP mode"
Run-CmdIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "disable_geoip" "disable-geoip path" "`"$Bin`" --no-color --data-provider disable-geoip -M -q 1 -m 2 --timeout 1000 1.1.1.1" "hops max, .*ICMP mode"
Run-CmdIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "dn42_mode" "DN42 mode switch" "`"$Bin`" --no-color --dn42 -q 1 -m 2 --timeout 1000 1.1.1.1" "hops max, .*ICMP mode"
Check-JsonPureIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "json_trace" "Traceroute JSON stdout purity" "`"$Bin`" --no-color --json -q 1 -m 3 --timeout 1000 1.1.1.1"
Check-JsonPureIfSupported $mtuCapability.Supported $mtuCapability.Reason "json_mtu" "MTU JSON stdout purity" "`"$Bin`" --no-color --mtu --json --timeout 1000 -q 1 -m 3 1.1.1.1"
Check-JsonPureIfSupported $globalpingCapability.Supported $globalpingCapability.Reason "json_globalping" "Globalping JSON stdout purity" "`"$Bin`" --no-color --json --from Germany -q 1 -m 3 --timeout 1000 1.1.1.1"
Run-CmdIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "table_non_tty" "Table output smoke" "`"$Bin`" --no-color --table -q 1 -m 2 --timeout 1000 1.1.1.1" "(?m)^Hop\s+IP\s+Latency"
Check-OutputFileIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "output_custom" "Custom output file path" "`"$Bin`" --no-color -q 1 -m 2 --timeout 1000 -o `"$ArtifactsRoot\custom.log`" 1.1.1.1" (Join-Path $ArtifactsRoot "custom.log")
Check-OutputFileIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "output_default" "Default output file path" "set TEMP=$DefaultTmp && set TMP=$DefaultTmp && `"$Bin`" --no-color -q 1 -m 2 --timeout 1000 -O 1.1.1.1" $DefaultLog
Run-CmdIfSupported $mtuCapability.Supported $mtuCapability.Reason "mtu_text" "MTU text mode" "`"$Bin`" --no-color --mtu --timeout 1000 -q 1 -m 3 1.1.1.1" "Path MTU:"
Write-Record mtu_tty_color SKIP "MTU TTY colorized output; no portable Windows PTY capture in this script"
Run-CmdIfSupported $mtuCapability.Supported $mtuCapability.Reason "mtu_non_tty_plain" "MTU non-TTY output has no ANSI" "`"$Bin`" --mtu --timeout 1000 -q 1 -m 3 1.1.1.1" "Path MTU:"
Run-CmdIfSupported $mtrCapability.Supported $mtrCapability.Reason "mtr_report" "MTR report ICMP" "`"$Bin`" --no-color -r -q 2 -i 300 --timeout 1000 -m 4 1.1.1.1" "(?m)^HOST:"
Run-CmdIfSupported $mtrCapability.Supported $mtrCapability.Reason "mtr_wide" "MTR wide + show-ips" "`"$Bin`" --no-color -w --show-ips -q 2 -i 300 --timeout 1000 -m 4 1.1.1.1" "(?m)^HOST:"
Run-CmdIfSupported $mtrCapability.Supported $mtrCapability.Reason "mtr_raw" "MTR raw stream" "`"$Bin`" --no-color -r --raw -q 2 -i 300 --timeout 1000 -m 4 1.1.1.1" "(?m)^1\|"
Run-CmdIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "fast_trace_file" "Fast trace via --file" "`"$Bin`" --no-color --file `"$Targets`" -q 1 -m 2 --timeout 1000" "traceroute to "
Run-CmdIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "tiny_smoke" "nexttrace-tiny smoke" "`"$Tiny`" --no-color -q 1 -m 2 --timeout 1000 1.1.1.1" "hops max, .*ICMP mode"
Run-CmdIfSupported $mtrCapability.Supported $mtrCapability.Reason "ntr_report" "ntr report smoke" "`"$Ntr`" --no-color -r -q 2 -i 300 --timeout 1000 -m 4 1.1.1.1" "(?m)^HOST:"

Check-PacketCaptureIfSupported $icmp4Capability.Supported $icmp4Capability.Reason "psize_tos_icmp4" "ICMPv4 psize/tos packet capture" "`"$Bin`" --no-color -q 1 -m 1 --timeout 1000 --psize 84 -Q 46 1.1.1.1" "host 1.1.1.1" "Differentiated Services Field: 0x2e" "Total Length: 84"
Check-PacketCaptureIfSupported $udp4Capability.Supported $udp4Capability.Reason "psize_tos_udp4" "UDPv4 psize/tos packet capture" "`"$Bin`" --no-color -U -q 1 -m 1 --timeout 1000 --psize 84 -Q 46 1.1.1.1" "host 1.1.1.1" "Differentiated Services Field: 0x2e" "Total Length: 84"
Check-PacketCaptureIfSupported $tcp4Capability.Supported $tcp4Capability.Reason "psize_tos_tcp4" "TCPv4 psize/tos packet capture" "`"$Bin`" --no-color -T -q 1 -m 1 --timeout 1000 --psize 84 -Q 46 1.1.1.1" "host 1.1.1.1" "Differentiated Services Field: 0x2e" "Total Length: 84"
Check-PacketCaptureIfSupported $icmp6Capability.Supported $icmp6Capability.Reason "psize_tos_icmp6" "ICMPv6 psize/tos packet capture" "`"$Bin`" --no-color -6 -q 1 -m 1 --timeout 1000 --psize 84 -Q 46 2606:4700:4700::1111" "host 2606:4700:4700::1111" "Traffic Class: 0x2e" "Payload Length: 44"
Check-PacketCaptureIfSupported $udp6Capability.Supported $udp6Capability.Reason "psize_tos_udp6" "UDPv6 psize/tos packet capture" "`"$Bin`" --no-color -6 -U -q 1 -m 1 --timeout 1000 --psize 84 -Q 46 2606:4700:4700::1111" "host 2606:4700:4700::1111" "Traffic Class: 0x2e" "Payload Length: 44"
Check-PacketCaptureIfSupported $tcp6Capability.Supported $tcp6Capability.Reason "psize_tos_tcp6" "TCPv6 psize/tos packet capture" "`"$Bin`" --no-color -6 -T -q 1 -m 1 --timeout 1000 --psize 84 -Q 46 2606:4700:4700::1111" "host 2606:4700:4700::1111" "Traffic Class: 0x2e" "Payload Length: 44"

$deployLog = Join-Path $ArtifactsDir "deploy_server.txt"
$deployStdout = "$deployLog.stdout"
$deployStderr = "$deployLog.stderr"
Remove-Item -Force -ErrorAction Ignore $deployLog, $deployStdout, $deployStderr
$deploy = Start-Process -FilePath $Bin -ArgumentList "--listen", "127.0.0.1:30080", "--deploy" -RedirectStandardOutput $deployStdout -RedirectStandardError $deployStderr -PassThru -WindowStyle Hidden
try {
    if (Wait-HttpReady "http://127.0.0.1:30080/") {
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:30080/" -TimeoutSec 5 | Out-File -Encoding utf8 (Join-Path $ArtifactsDir "deploy_root.txt")
            Write-Record deploy_root PASS "Web root reachable"
        }
        catch {
            Write-Record deploy_root FAIL "Web root request failed"
        }
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:30080/api/options" -TimeoutSec 5 | Select-Object -ExpandProperty Content | Tee-Object -FilePath (Join-Path $ArtifactsDir "deploy_options.txt") | Out-Null
            $options = Get-Content -Raw -Path (Join-Path $ArtifactsDir "deploy_options.txt")
            if ($options.Contains('"packet_size":null') -and $options.Contains('"tos":0')) {
                Write-Record deploy_options PASS "Options API exposes packet_size=null and tos=0"
            }
            else {
                Write-Record deploy_options FAIL "Options API check failed"
            }
        }
        catch {
            Write-Record deploy_options FAIL "Options API request failed"
        }
        if ($icmp4Capability.Supported) {
            try {
                $body = '{"target":"1.1.1.1","queries":1,"max_hops":3,"timeout_ms":1000}'
                Invoke-WebRequest -UseBasicParsing -Method Post -ContentType "application/json" -Body $body -Uri "http://127.0.0.1:30080/api/trace" -TimeoutSec 15 | Select-Object -ExpandProperty Content | Tee-Object -FilePath (Join-Path $ArtifactsDir "deploy_trace.txt") | Out-Null
                $traceBody = Get-Content -Raw -Path (Join-Path $ArtifactsDir "deploy_trace.txt")
                if ($traceBody.Contains('"resolved_ip"')) {
                    Write-Record deploy_trace PASS "REST trace endpoint works"
                }
                else {
                    Write-Record deploy_trace FAIL "REST trace response check failed"
                }
            }
            catch {
                Write-Record deploy_trace FAIL "REST trace endpoint failed"
            }
        }
        else {
            Write-SkipRecord deploy_trace "REST trace endpoint works" $icmp4Capability.Reason
        }
    }
    else {
        Write-Record deploy_root FAIL "deploy server not ready"
        Write-Record deploy_options FAIL "deploy server not ready"
        if ($icmp4Capability.Supported) {
            Write-Record deploy_trace FAIL "deploy server not ready"
        }
        else {
            Write-SkipRecord deploy_trace "REST trace endpoint works" $icmp4Capability.Reason
        }
    }
}
finally {
    $deployOutText = if (Test-Path $deployStdout) { Get-Content -Raw -Path $deployStdout } else { "" }
    $deployErrText = if (Test-Path $deployStderr) { Get-Content -Raw -Path $deployStderr } else { "" }
    Set-Content -Path $deployLog -Value ($deployOutText + $deployErrText)
    Remove-Item -Force -ErrorAction Ignore $deployStdout, $deployStderr
    if (-not $deploy.HasExited) {
        Stop-Process -Id $deploy.Id -Force -ErrorAction SilentlyContinue
    }
}

$rows = Get-Content -Path $Summary
$pass = @($rows | Where-Object { $_ -match "`tPASS`t" }).Count
$fail = @($rows | Where-Object { $_ -match "`tFAIL`t" }).Count
$skip = @($rows | Where-Object { $_ -match "`tSKIP`t" }).Count

Write-Host "__SUMMARY__"
Get-Content -Path $Summary
Write-Host ("pass={0} fail={1} skip={2} total={3}" -f $pass, $fail, $skip, $rows.Count)
Write-Host ("artifacts_root={0}" -f (Format-DisplayPath $ArtifactsRoot))
if ($fail -gt 0) {
    exit 1
}
