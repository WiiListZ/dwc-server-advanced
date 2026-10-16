# Payload and Certificate Setup

This guide explains the complete payload path:

1. Configure the server.
2. Generate the HTTPS certificates the client needs.
3. Build the client payload files with `dwc-server-advanced-patcher`.
4. Copy the exact generated files into the server folder (`payload/`).
5. Configure the client correctly.
6. Verify the endpoints and fix common errors.

The examples use these local paths:

```text
C:\dwc\dwc-server-advanced
C:\dwc\dwc-server-advanced-patcher
```

Replace them if your folders are different.

## How The Payload Flow Works

The payload flow has three parts:

```text
Stage 0: client redirection
  The game is routed to this server through DNS, hosts entries, or a manual patch.

Stage 1: generic downloader
  The client downloads /w0 or /w1 from the NAS host. The server reads payload/stage1.bin
  or payload/stage1v1.bin.

Stage 2: game payload
  Stage 1 downloads /payload?g=<game-version>&s=<salt>&h=<hash>. The server reads
  payload/binary/payload.<game-version>.bin, patches the salt hash, signs it with
  payload/private-key.pem, and returns the signed payload.
```

For Mario Kart Wii there is one more file:

```text
QR2 SBCM exploit
  When a client needs patching, the server sends payload/sbcm/payload.RMC*.bin during
  the QR2 matchmaking flow. That exploit starts the stage 1 download.
```

The stage 2 payload files and `payload/private-key.pem` must come from the same patcher build. If the key and payload binaries do not match, the client rejects the payload.

The owner can edit the Mario Kart Wii welcome popup under **Panel → Server → Settings → General**. Turning it off omits `wl:motd` from the login response. Build and install a current patcher payload on clients to honor this setting; older payload binaries display their built-in fallback message.

## Choose One Client Mode

Do not mix these modes for the same client.

The owner can enable **Panel → Server → Settings → Clients → Require payload features for Wii clients** (`requirePayloadFeatures: true`). In this mode a Wii client must present a valid payload version and signature at GPCM login. DNS-only and NoSSL clients cannot connect, even if `allowNoSSLCheatClients` is enabled. Leave it off to accept both payload and compatible non-payload clients. DS logins are unaffected. Install the patch and server payload files before enabling it for your players.

Native login messages can be sent only to online players with payload features. The panel hides the Message action for an online player without them; the API also rejects that request. An offline player's message can still be queued for their next payload login.

### Mode A: Manually Redirected Or NoSSL Client

Use this for local Dolphin testing when the client already reaches the server through hosts entries, NoSSL codes, or a manual patch and you do not want the server to inject the QR2 SBCM exploit.

```yaml
allowNoSSLCheatClients: true
payloadServerAddress: ""
```

This mode is simpler, but payload-only features such as native custom login popups are not available unless your client has compatible payload support already.

### Mode B: Server Injects The Payload

Use this when the client is routed to the server but should receive the payload from the server during login/matchmaking.

```yaml
allowNoSSLCheatClients: false
payloadServerAddress: ""
```

The client must not also be using a NoSSL/manual patch that already bypasses the same flow. If it is already patched and the server injects `payload/sbcm/payload.RMCP.bin` anyway, Mario Kart Wii can disconnect when entering Worldwide.

### Mode C: External Payload Server

Use this only if another service handles `/payload`.

```yaml
allowNoSSLCheatClients: false
payloadServerAddress: 127.0.0.1:29997
```

Important details:

- This server still serves `/w0` and `/w1`, so `payload/stage1.bin` still must exist next to `dwc-server-advanced.exe`.
- `/payload` is forwarded to `payloadServerAddress`.
- The external payload server must be compatible with this server's signing flow. A static file host is not enough when the client sends `s=` and `h=`, because the response must be salted and signed per request.

For most setups, leave `payloadServerAddress` blank and use the integrated payload handler.

## Server File Layout

The final server layout must be:

```text
server folder/
  dwc-server-advanced.exe
  config.yml
  fullchain.pem
  privkey.pem
  naswii-chain.der
  naswii-signed-key.pem
  payload/
    private-key.pem
    stage1.bin
    stage1v1.bin              needed by the generated Gecko patch (/w1)
    sbcm/
      payload.RMCE.bin
      payload.RMCJ.bin
      payload.RMCK.bin
      payload.RMCP.bin
    binary/
      payload.RMCED00.bin
      payload.RMCEN0001.bin
      payload.RMCJD00.bin
      payload.RMCJN0001.bin
      payload.RMCKD00.bin
      payload.RMCKN0001.bin
      payload.RMCPD00.bin
      payload.RMCPN0001.bin
      ...other generated payload files...
```

Mario Kart Wii region codes:

```text
RMCE = USA
RMCP = Europe/PAL
RMCJ = Japan
RMCK = Korea
```

File rules:

- `payload/sbcm/payload.RMCP.bin` is the QR2 SBCM exploit.
- `payload/binary/payload.RMCPD00.bin` is the final stage 2 payload.
- Never put `payload.RMCPD00.bin` in `payload/sbcm`.
- Never put `payload.RMCP.bin` in `payload/binary`.
- Copy all payload files from the same `dwc-server-advanced-patcher` build.
- The generated Gecko patch requests `/w1`, so copy `dist/stage1.bin` to `payload/stage1v1.bin` too. `/w0` alone returning HTTP 200 does not verify the Gecko path.

The server has a guard for the most common mistake: if a stage 2 file is placed in `payload/sbcm`, the log says it expected the SBCM exploit from `dwc-server-advanced-patcher --exploit`.

## Configure The Server

For a local Dolphin payload test on this PC:

```yaml
gsAddress: 127.0.0.1
nasAddress: 127.0.0.1
nasAddressHttps: 127.0.0.1
nasPort: 80
nasPortHttps: 443

enableHttps: true
certPath: fullchain.pem
keyPath: privkey.pem

enableHttpsExploitWii: true
certDerPathWii: naswii-chain.der
keyPathWii: naswii-signed-key.pem

allowDefaultDolphinKeys: true
allowNoSSLCheatClients: false
payloadServerAddress: ""
```

For a manually patched or NoSSL local client, switch only this:

```yaml
allowNoSSLCheatClients: true
```

For a remote server:

```yaml
gsAddress: 0.0.0.0
nasAddress: 0.0.0.0
nasAddressHttps: 0.0.0.0
enableHttps: true
enableHttpsExploitWii: true
allowNoSSLCheatClients: false
payloadServerAddress: ""
```

Open the required TCP/UDP ports on the firewall. At minimum this server uses NAS HTTP/HTTPS, GPCM/GPSP/serverbrowser/gamestats TCP ports, QR2/NATNEG UDP ports, and PostgreSQL locally.

## Generate Certificates

The server uses three different key/certificate groups:

```text
fullchain.pem / privkey.pem
  Normal HTTPS server certificate. Used by the Go TLS path.

naswii-chain.der / naswii-signed-key.pem
  Wii HTTPS exploit certificate chain. Used when enableHttpsExploitWii is true.

payload/private-key.pem
  Payload signing key generated by dwc-server-advanced-patcher. This is not a TLS key.
```

Do not commit generated keys or certificates.

### Normal HTTPS Certificate

For local Dolphin testing, a self-signed certificate is enough if Dolphin has SSL verification disabled or if you are using the Wii HTTPS exploit path.

Run from the server folder:

```powershell
openssl req -x509 -newkey rsa:2048 -nodes `
  -keyout privkey.pem `
  -out fullchain.pem `
  -days 3650 `
  -subj "/CN=naswii.nintendowifi.net" `
  -sha256
```

Configure:

```yaml
enableHttps: true
certPath: fullchain.pem
keyPath: privkey.pem
```

For a public web domain, use a real certificate for your domain instead. Example on Linux:

```bash
sudo certbot certonly --standalone -d wfc.example.com
sudo cp /etc/letsencrypt/live/wfc.example.com/fullchain.pem ./fullchain.pem
sudo cp /etc/letsencrypt/live/wfc.example.com/privkey.pem ./privkey.pem
sudo chown "$USER":"$USER" fullchain.pem privkey.pem
```

The game still asks for Nintendo hostnames such as `naswii.nintendowifi.net`; routing must point those hostnames to your server.

### Wii HTTPS Exploit Certificate

Use this when:

```yaml
enableHttpsExploitWii: true
```

For Dolphin, the required CA files are normally here:

```text
%APPDATA%\Dolphin Emulator\Wii
```

Required files:

```text
clientca.pem
clientcakey.pem
rootca.pem
```

Run from the server folder:

```powershell
$wii = "$env:APPDATA\Dolphin Emulator\Wii"

openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:1024 -out naswii-signed-key.pem

openssl req -new `
  -key naswii-signed-key.pem `
  -out naswii-signed.csr `
  -subj "/C=US/ST=Washington/L=Redmond/O=Nintendo of America Inc./OU=Nintendo Wifi Network/CN=naswii.nintendowifi.net"

openssl x509 -req `
  -in naswii-signed.csr `
  -CA "$wii\clientca.pem" `
  -CAkey "$wii\clientcakey.pem" `
  -set_serial 87 `
  -out naswii-signed-cert.pem `
  -days 3650 `
  -sha1

openssl x509 -in naswii-signed-cert.pem -outform DER -out naswii-signed-cert.der
openssl x509 -in "$wii\clientca.pem" -outform DER -out naswii-clientca.der

$server = [System.IO.File]::ReadAllBytes((Join-Path (Get-Location) "naswii-signed-cert.der"))
$client = [System.IO.File]::ReadAllBytes((Join-Path (Get-Location) "naswii-clientca.der"))
[System.IO.File]::WriteAllBytes((Join-Path (Get-Location) "naswii-chain.der"), $server + $client)
```

Configure:

```yaml
enableHttpsExploitWii: true
certDerPathWii: naswii-chain.der
keyPathWii: naswii-signed-key.pem
```

Why `-set_serial 87` is used: OpenSSL otherwise tries to create a serial file next to Dolphin's CA in AppData, which often fails or dirties the Dolphin directory.

### Certificate Verification

Normal HTTPS cert/key must match:

```powershell
openssl x509 -in fullchain.pem -noout -modulus | openssl md5
openssl rsa -in privkey.pem -noout -modulus | openssl md5
```

The MD5 outputs must be the same.

Wii exploit cert/key must match:

```powershell
openssl x509 -in naswii-signed-cert.pem -noout -modulus | openssl md5
openssl rsa -in naswii-signed-key.pem -noout -modulus | openssl md5
```

The MD5 outputs must be the same.

Check the Wii exploit cert subject and issuer:

```powershell
openssl x509 -in naswii-signed-cert.pem -noout -subject -issuer -serial
```

Expected shape:

```text
subject=... CN=naswii.nintendowifi.net
issuer=... CN=Wii NWC Prod 1 ...
serial=57
```

`57` is hexadecimal for decimal serial `87`.

## Build Payloads With dwc-server-advanced-patcher

### Build Requirements

Use Linux, WSL2, or a devkitPro-capable shell. Windows PowerShell alone is not enough for the patcher build script.

Required tools:

```text
Python 3.11+
Python package: cryptography
devkitPro / devkitPPC
make and standard build tools
```

On devkitPro/MSYS2 or Linux, install the Python module:

```bash
python3 -m pip install cryptography
```

### Pick The Build Domain

The build domain is passed as `WWFC_DOMAIN`. It must be the host name or IP the client will use to reach the NAS/payload server.

Local Dolphin on the same PC:

```text
127.0.0.1
```

Remote server:

```text
wfc.example.com
```

Do not include `http://`, `https://`, or a path.

### Clean Build

From the patcher directory:

```bash
cd /mnt/c/dwc/dwc-server-advanced-patcher
./make.sh --clean
./make.sh --clean-keys
./make.sh --keys
./make.sh --all --exploit -- -j8 -DWWFC_DOMAIN=\"127.0.0.1\"
```

For a remote server:

```bash
./make.sh --clean
./make.sh --clean-keys
./make.sh --keys
./make.sh --all --exploit -- -j8 -DWWFC_DOMAIN=\"wfc.example.com\"
```

What these commands create:

```text
dwc-server-advanced-patcher/dist/private-key.pem
dwc-server-advanced-patcher/dist/stage1.bin
dwc-server-advanced-patcher/dist/binary/payload.*.bin
dwc-server-advanced-patcher/exploit/sbcm/payload.RMCE.bin
dwc-server-advanced-patcher/exploit/sbcm/payload.RMCJ.bin
dwc-server-advanced-patcher/exploit/sbcm/payload.RMCK.bin
dwc-server-advanced-patcher/exploit/sbcm/payload.RMCP.bin
dwc-server-advanced-patcher/patch/build/*.txt
```

`patch/build/*.txt` are client-side Gecko/manual patch files. They are not copied into the server's `payload/` folder.

## Copy Payloads Into The Server

Run in PowerShell:

```powershell
$Server = "C:\dwc\dwc-server-advanced"
$Patcher = "C:\dwc\dwc-server-advanced-patcher"

New-Item -ItemType Directory -Force `
  "$Server\payload" `
  "$Server\payload\binary" `
  "$Server\payload\sbcm" | Out-Null

Copy-Item "$Patcher\dist\private-key.pem" "$Server\payload\private-key.pem" -Force
Copy-Item "$Patcher\dist\stage1.bin" "$Server\payload\stage1.bin" -Force
Copy-Item "$Patcher\dist\stage1.bin" "$Server\payload\stage1v1.bin" -Force
Copy-Item "$Patcher\dist\binary\*.bin" "$Server\payload\binary\" -Force
Copy-Item "$Patcher\exploit\sbcm\payload.*.bin" "$Server\payload\sbcm\" -Force
```

Validate the required Mario Kart Wii files:

```powershell
$Required = @(
  "payload\private-key.pem",
  "payload\stage1.bin",
  "payload\stage1v1.bin",
  "payload\sbcm\payload.RMCE.bin",
  "payload\sbcm\payload.RMCJ.bin",
  "payload\sbcm\payload.RMCK.bin",
  "payload\sbcm\payload.RMCP.bin",
  "payload\binary\payload.RMCED00.bin",
  "payload\binary\payload.RMCEN0001.bin",
  "payload\binary\payload.RMCJD00.bin",
  "payload\binary\payload.RMCJN0001.bin",
  "payload\binary\payload.RMCKD00.bin",
  "payload\binary\payload.RMCKN0001.bin",
  "payload\binary\payload.RMCPD00.bin",
  "payload\binary\payload.RMCPN0001.bin"
)

Set-Location "C:\dwc\dwc-server-advanced"
$Required | ForEach-Object {
  if (Test-Path $_) { "OK $_" } else { "MISSING $_" }
}
```

## Configure The Client

### Local Dolphin, No Payload Injection

Use this when `allowNoSSLCheatClients` is `true`.

Client requirements:

- Hosts file maps Nintendo WFC hosts to `127.0.0.1`.
- Dolphin can use default device keys if `<allowDefaultDolphinKeys>true</allowDefaultDolphinKeys>`.
- SSL verification can be disabled for simple local testing.
- Do not expect payload-only login popups.

Server setting:

```yaml
allowNoSSLCheatClients: true
```

### Local Dolphin, Server Payload Injection

Use this when `allowNoSSLCheatClients` is `false`.

Client requirements:

- Hosts file maps Nintendo WFC hosts to `127.0.0.1`.
- Do not use a NoSSL/manual patch for the same connection flow.
- Use the certificate setup above.
- Enter Worldwide/Regional/Friends so QR2 can deliver the SBCM exploit.

Server setting:

```yaml
allowNoSSLCheatClients: false
```

### Real Wii Or Remote Clients

Client requirements:

- DNS must route Nintendo WFC hostnames to your server IP.
- TCP/UDP ports must be reachable from the client.
- Use the Wii HTTPS exploit cert path unless the client is otherwise patched to trust/reach your server.
- Build payloads with the domain/IP clients use to reach your service.

Useful hostnames to route:

```text
naswii.nintendowifi.net
nas.nintendowifi.net
conntest.nintendowifi.net
gamestats2.gs.nintendowifi.net
gpcm.gs.nintendowifi.net
gpsp.gs.nintendowifi.net
sake.gs.nintendowifi.net
```

Your exact hostname list can vary by game. For Mario Kart Wii, make sure NAS, GPCM/GPSP, gamestats, sake, QR2, NATNEG, and serverbrowser traffic all reaches this server.

## Rebuild And Restart The Server

After changing config, certificates, or payload files:

```powershell
Set-Location "C:\dwc\dwc-server-advanced"
go build -o dwc-server-advanced.exe .
Get-Process dwc-server-advanced -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Process -FilePath .\dwc-server-advanced.exe -WorkingDirectory . -WindowStyle Hidden
```

If PowerShell reports no `dwc-server-advanced` process during `Stop-Process`, that is fine.

## Verify Endpoints

Run these from the server folder while the server is running.

Stage 1:

```powershell
$r = Invoke-WebRequest `
  -UseBasicParsing `
  -Headers @{Host="naswii.nintendowifi.net"} `
  -Uri "http://127.0.0.1/w1"

"HTTP $($r.StatusCode) length=$($r.RawContentLength)"
```

Expected:

```text
HTTP 200 length=3550
```

The exact length can change if `stage1.bin` changes. It should be `2 + stage1v1.bin size`. Test `/w0` as well if you use the SBCM injection path.

Final payload:

```powershell
$r = Invoke-WebRequest `
  -UseBasicParsing `
  -Headers @{Host="naswii.nintendowifi.net"} `
  -Uri "http://127.0.0.1/payload?g=RMCPD00"

"HTTP $($r.StatusCode) length=$($r.RawContentLength)"
```

Expected for the current Mario Kart Wii PAL Dolphin payload:

```text
HTTP 200 length=22624
```

If this fails, check:

- `payload/binary/payload.RMCPD00.bin` exists.
- `payload/private-key.pem` exists and came from the same patcher build.
- The server was restarted after copying files.

HTTPS NAS:

```powershell
Invoke-WebRequest `
  -UseBasicParsing `
  -SkipCertificateCheck `
  -Headers @{Host="naswii.nintendowifi.net"} `
  -Uri "https://127.0.0.1/ac"
```

A certificate warning or NAS method error is less important than proving the HTTPS listener is reachable. A connection refusal means the server is not listening on HTTPS or the port is blocked.

## Expected Server Logs

In payload injection mode, a successful path should contain lines like:

```text
NAS: Starting HTTP server on 127.0.0.1:80
NAS: Starting HTTPS server on 127.0.0.1:443
QR2: Command: AVAILABLE
Sending SBCM exploit to DNS patcher client
Loaded SBCM exploit: payload\sbcm\payload.RMCP.bin
Command: CLIENT_EXPLOIT_ACK
```

The server should not say it loaded:

```text
payload.RMCPD00.bin
```

from `payload/sbcm`. That file belongs in `payload/binary`.

## Common Errors

`open privkey.pem: The system cannot find the file specified`

`certPath` or `keyPath` points to a missing normal HTTPS file.

`panic: open naswii-chain.der: The system cannot find the file specified`

`enableHttpsExploitWii` is true, but `certDerPathWii` is missing or wrong.

`Failed to parse server certs`

The file configured in `certPath` is not a valid PEM certificate, or it does not match `keyPath`.

`Failed to read payload file`

The requested stage 2 file is missing. For PAL Mario Kart Wii Dolphin, check:

```text
payload/binary/payload.RMCPD00.bin
```

`Salt hash mismatch`

The client requested `/payload` with a salt/hash pair the server did not accept. Fixes:

- Rebuild the server from current source.
- Rebuild all payload files with one clean patcher build.
- Copy `private-key.pem`, `stage1.bin`, `dist/binary/*`, and `exploit/sbcm/*` from that same build.
- Make sure no proxy rewrites the `/payload?...` query string.

`Client session shutdown` right after `CLIENT_EXPLOIT_ACK`

The client probably should not receive SBCM injection. For a manually patched/NoSSL local Dolphin client:

```yaml
allowNoSSLCheatClients: true
```

`20100` after `QR2: AVAILABLE`

The client reached availability but did not complete the full login. Check:

- Hosts/DNS routes.
- HTTPS port 443.
- `fullchain.pem` / `privkey.pem`.
- `naswii-chain.der` / `naswii-signed-key.pem` if the exploit path is enabled.
- Dolphin SSL verification settings if using local self-signed TLS.

`23404`, `20917`, or blank custom error screen

These usually mean the client reached the server but the payload/auth state is not what the game expects. Check:

- The client mode matches `allowNoSSLCheatClients`.
- The payload files are from one build.
- The client is not using both NoSSL/manual patch and server-side SBCM injection.
- The game ID has a matching file in `payload/binary`.
- The server was restarted after copying payload files.

## Known Good Local Setup

For the current local Dolphin setup on this PC:

```yaml
serverName: DWC Server Advanced
gsAddress: 127.0.0.1
nasAddress: 127.0.0.1
nasAddressHttps: 127.0.0.1
enableHttps: true
enableHttpsExploitWii: true
certPath: fullchain.pem
keyPath: privkey.pem
certDerPathWii: naswii-chain.der
keyPathWii: naswii-signed-key.pem
allowDefaultDolphinKeys: true
allowNoSSLCheatClients: true
payloadServerAddress: ""
```

Use `allowNoSSLCheatClients>false` only when you want this server to inject the SBCM payload and the client is not already manually patched for the same flow.
