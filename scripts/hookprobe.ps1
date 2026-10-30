param([string]$Name)
$raw = [Console]::In.ReadToEnd()
$j = $raw | ConvertFrom-Json
$cmd = $j.tool_input.command
$resp = if ($j.tool_response) { ($j.tool_response | ConvertTo-Json -Compress).Length } else { -1 }
$line = "{0:HH:mm:ss.fff} {1} cmd=[{2}] response_len={3}" -f (Get-Date), $Name, $cmd, $resp
Add-Content -Path "$env:HOOKPROBE_LOG" -Value $line
