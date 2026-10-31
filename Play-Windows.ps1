param([switch]$Background,[switch]$NoBrowser)
$ErrorActionPreference='Stop'
function Get-StarsSha256([string]$filename){
 $stream=[IO.File]::OpenRead($filename);$sha=[Security.Cryptography.SHA256]::Create()
 try{return [BitConverter]::ToString($sha.ComputeHash($stream)).Replace('-','').ToLowerInvariant()}finally{$stream.Dispose();$sha.Dispose()}
}
$projectRoot=$PSScriptRoot
$backendExe=Join-Path $projectRoot 'windows\stars-local.exe'
if(Test-Path -LiteralPath (Join-Path $projectRoot 'dist\stars-local.pending.exe')) {$backendExe=Join-Path $projectRoot 'dist\stars-local.exe'}
$webScript=Join-Path $projectRoot 'tools\local-web.mjs'
$logs=Join-Path $projectRoot 'tmp\launch'
$null=New-Item -ItemType Directory -Force -Path $logs
if((!(Test-Path -LiteralPath $backendExe) -and !(Test-Path -LiteralPath (Join-Path $projectRoot 'dist\stars-local.pending.exe'))) -or !(Test-Path -LiteralPath (Join-Path $projectRoot 'frontend\build\index.html'))) {throw 'Download the complete tomek-stars Windows edition, including windows/stars-local.exe and frontend/build.'}
$nodeCommand=Get-Command node.exe -ErrorAction SilentlyContinue
if($nodeCommand){try{if([version](& $nodeCommand.Source -p 'process.versions.node') -lt [version]'22.17.0'){$nodeCommand=$null}}catch{$nodeCommand=$null}}
$nodeExe=if($nodeCommand){$nodeCommand.Source}else{Join-Path $projectRoot '.runtime\node\node.exe'}
if(!(Test-Path -LiteralPath $nodeExe)) {
 Write-Host 'Downloading a portable Node.js LTS runtime into this game folder...'
 $releases=Invoke-RestMethod 'https://nodejs.org/dist/index.json'
 $release=$releases | Where-Object {$_.lts -and ([version]$_.version.TrimStart('v')) -ge [version]'22.17.0' -and $_.files -contains 'win-x64-zip'} | Select-Object -First 1
 if(!$release){throw 'Could not find a compatible Node.js LTS runtime.'}
 $runtimeRoot=Join-Path $projectRoot '.runtime';$null=New-Item -ItemType Directory -Force -Path $runtimeRoot
 $filename='node-'+$release.version+'-win-x64.zip';$zip=Join-Path $runtimeRoot $filename
 $base='https://nodejs.org/dist/'+$release.version+'/'
 Invoke-WebRequest ($base+$filename) -OutFile $zip -UseBasicParsing
 $checksums=(Invoke-WebRequest ($base+'SHASUMS256.txt') -UseBasicParsing).Content
 $expected=($checksums -split "`n" | Where-Object {$_ -match ('\s'+[regex]::Escape($filename)+'\s*$')}) -replace '\s+.*$',''
 if(!$expected -or (Get-StarsSha256 $zip) -ne $expected.Trim()) {throw 'Node.js download checksum mismatch.'}
 Add-Type -AssemblyName System.IO.Compression.FileSystem
 $extractRoot=Join-Path $runtimeRoot ([Guid]::NewGuid().ToString('N'));$null=New-Item -ItemType Directory -Force -Path $extractRoot
 [IO.Compression.ZipFile]::ExtractToDirectory($zip,$extractRoot)
 $expanded=Join-Path $extractRoot ('node-'+$release.version+'-win-x64')
  $portableDir=Join-Path $runtimeRoot 'node';$null=New-Item -ItemType Directory -Force -Path $portableDir
 Copy-Item -LiteralPath (Join-Path $expanded 'node.exe') -Destination (Join-Path $portableDir 'node.exe') -Force
 Copy-Item -LiteralPath (Join-Path $expanded 'LICENSE') -Destination (Join-Path $portableDir 'LICENSE') -Force
 $nodeExe=Join-Path $portableDir 'node.exe'
}
$version=& $nodeExe -p 'process.versions.node'
if([version]$version -lt [version]'22.17.0'){throw 'Node.js 22.17 or newer is required.'}
# Serve an immutable UI copy so later source edits/builds cannot break a running game.
$buildRoot=Join-Path $projectRoot 'frontend\build'
$uiVersion=(Get-StarsSha256 (Join-Path $buildRoot 'index.html')).Substring(0,16)
$uiRoot=Join-Path $projectRoot ('.runtime\ui\'+$uiVersion)
if(!(Test-Path -LiteralPath (Join-Path $uiRoot '.ready'))){
 $null=New-Item -ItemType Directory -Force -Path $uiRoot
 Get-ChildItem -LiteralPath $buildRoot -Force | Copy-Item -Destination $uiRoot -Recurse -Force
 Set-Content -LiteralPath (Join-Path $uiRoot '.ready') -Value $uiVersion
}
# Stop only listeners whose executable and command line belong to this copy.
foreach($port in @(8080,5173)) {
 $owners=@(Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique)
 foreach($processId in $owners){
  $process=Get-Process -Id $processId -ErrorAction Stop
  $details=Get-CimInstance Win32_Process -Filter "ProcessId = $processId"
  $knownBackends=@((Join-Path $projectRoot 'dist\stars-local.exe'),(Join-Path $projectRoot 'windows\stars-local.exe'))
  $owned=($port -eq 8080 -and $knownBackends -contains $process.Path) -or ($port -eq 5173 -and $process.ProcessName -eq 'node' -and ($details.CommandLine.Contains($webScript) -or $details.CommandLine.Contains((Join-Path $projectRoot 'frontend\node_modules\vite\bin\vite.js'))))
  if(!$owned){throw "Port $port belongs to another application; it was left running."}
  Stop-Process -Id $processId -ErrorAction Stop;$null=$process.WaitForExit(5000)
 }
}
if(Test-Path -LiteralPath (Join-Path $projectRoot 'dist\stars-local.pending.exe')) {Move-Item -LiteralPath (Join-Path $projectRoot 'dist\stars-local.pending.exe') -Destination (Join-Path $projectRoot 'dist\stars-local.exe') -Force}
$backend=$null;$frontend=$null
try{
 $backend=Start-Process -FilePath $backendExe -ArgumentList 'serve' -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $logs 'backend.out.log') -RedirectStandardError (Join-Path $logs 'backend.err.log')
 $oldPort=$env:STARS_HTTP_PORT;$oldBackend=$env:STARS_LOCAL_BACKEND;$oldWebRoot=$env:STARS_WEB_ROOT
 try{$env:STARS_WEB_ROOT=$uiRoot;$env:STARS_HTTP_PORT='5173';$env:STARS_LOCAL_BACKEND='http://127.0.0.1:8080';$frontend=Start-Process -FilePath $nodeExe -ArgumentList ('"'+$webScript+'"') -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $logs 'frontend.out.log') -RedirectStandardError (Join-Path $logs 'frontend.err.log')}finally{$env:STARS_HTTP_PORT=$oldPort;$env:STARS_LOCAL_BACKEND=$oldBackend;$env:STARS_WEB_ROOT=$oldWebRoot}
 $ready=$false
 for($attempt=0;$attempt -lt 60;$attempt++){
  if($backend.HasExited -or $frontend.HasExited){throw 'The game could not start. Check tmp/launch logs.'}
  try{if((Invoke-WebRequest 'http://127.0.0.1:5173/local-ready' -UseBasicParsing -TimeoutSec 1).Content -eq 'stars-local-singleplayer'){$probe=[Net.Sockets.TcpClient]::new('127.0.0.1',8080);$probe.Dispose();$ready=$true;break}}catch{}
  Start-Sleep -Milliseconds 250
 }
 if(!$ready){throw 'Local game startup timed out.'}
 @{backend=$backend.Id;frontend=$frontend.Id;webRoot=$uiRoot;started=[DateTime]::UtcNow.ToString('o')}|ConvertTo-Json|Set-Content -Encoding utf8 (Join-Path $logs 'instance.json')
 if(!$NoBrowser){Start-Process 'http://localhost:5173/local-start'}
 if($Background){$backend=$null;$frontend=$null;return}
 $null=Read-Host 'Press Enter to stop tomek-stars'
}finally{foreach($child in @($frontend,$backend)){if($null -ne $child -and !$child.HasExited){Stop-Process -Id $child.Id -ErrorAction SilentlyContinue}}}
