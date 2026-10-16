# HTTPS Certificates

With `enableHttps: true`, dwc-server-advanced creates the missing normal HTTPS certificate and key on startup. When `enableHttpsExploitWii` or `enableHttpsExploitDS` is true, it also creates the corresponding legacy chain from Dolphin's `clientca.pem` and `clientcakey.pem` if they are available. The files are written to the paths in `config.yml` without replacing existing files. In the web panel, open **Server → Settings → HTTPS**, save any changes, click **Generate certificate files**, then restart the server. The owner can also run `dwc-server-advanced --dir <server-folder> --generate-https` before starting the server. Set `DWC_DOLPHIN_WII_DIR` if Dolphin's `Wii` folder is in a custom location. The normal certificate is self-signed and intended for local testing; use a trusted certificate for a public site. If a file is missing or invalid, HTTPS reports an error while HTTP and the web panel remain available.

The commands below provide a manual alternative and explain each certificate set.

For the full payload setup sequence, including client mode selection, payload generation, file copying, and endpoint checks, see `docs/PAYLOADS.md`.

The server uses separate certificate/key sets for normal HTTPS, Wii and DS:

```text
fullchain.pem / privkey.pem
  Used by the normal Go TLS path. Dolphin can use this path when SSL verification is disabled.

naswii-chain.der / naswii-signed-key.pem
  Used by the custom Wii TLS exploit path when enableHttpsExploitWii is true.

nas-cert.der / nwc.der / nas-key.pem
  Used by the custom DS TLS exploit path when enableHttpsExploitDS is true.
```

`payload/private-key.pem` is separate. It signs payload binaries and must come from the same patcher build as the payload files.

All generated certificate and key files are local secrets. They are ignored by `.gitignore` and should not be committed.

## Local Dolphin Certificates

For local Dolphin testing, a self-signed TLS certificate is enough because Dolphin has SSL verification disabled in `Dolphin.ini`.

From the repository root:

```powershell
openssl req -x509 -newkey rsa:2048 -nodes -keyout privkey.pem -out fullchain.pem -days 3650 -subj "/CN=naswii.nintendowifi.net" -sha256
```

Configure:

```yaml
enableHttps: true
certPath: fullchain.pem
keyPath: privkey.pem
```

The current server code allows legacy TLS 1.0/1.1 and older RSA cipher suites on this path because Dolphin/Mario Kart Wii can offer only old TLS versions during NAS login.

## Wii HTTPS Exploit Chain

If `enableHttpsExploitWii` is true, generate a NASWII certificate signed by Dolphin's Wii client certificate and send it as a DER chain.

Dolphin's Wii cert files are usually in one of these folders:

```text
%APPDATA%\Dolphin Emulator\Wii
D:\dolphin-master-5.0-13603-x64\Dolphin Emulator\Wii
```

The required files are:

```text
clientca.pem
clientcakey.pem
rootca.pem
```

PowerShell commands:

```powershell
$wii = "$env:APPDATA\Dolphin Emulator\Wii"

openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:1024 -out naswii-signed-key.pem

openssl req -new -key naswii-signed-key.pem -out naswii-signed.csr -subj "/C=US/ST=Washington/L=Redmond/O=Nintendo of America Inc./OU=Nintendo Wifi Network/CN=naswii.nintendowifi.net"

openssl x509 -req -in naswii-signed.csr -CA "$wii\clientca.pem" -CAkey "$wii\clientcakey.pem" -set_serial 87 -out naswii-signed-cert.pem -days 3650 -sha1

openssl x509 -in naswii-signed-cert.pem -outform DER -out naswii-signed-cert.der
openssl x509 -in "$wii\clientca.pem" -outform DER -out naswii-clientca.der

$server = [System.IO.File]::ReadAllBytes((Join-Path (Get-Location) "naswii-signed-cert.der"))
$client = [System.IO.File]::ReadAllBytes((Join-Path (Get-Location) "naswii-clientca.der"))
[System.IO.File]::WriteAllBytes((Join-Path (Get-Location) "naswii-chain.der"), $server + $client)
```

Use `-set_serial 87` so OpenSSL does not try to create a serial file next to Dolphin's certificate in AppData.

Configure:

```yaml
enableHttpsExploitWii: true
certDerPathWii: naswii-chain.der
keyPathWii: naswii-signed-key.pem
```

## Verify Files

Check that the normal TLS cert and key match:

```powershell
openssl x509 -in fullchain.pem -noout -modulus | openssl md5
openssl rsa -in privkey.pem -noout -modulus | openssl md5
```

The two hashes should match.

Check that the NASWII signed cert and key match:

```powershell
openssl x509 -in naswii-signed-cert.pem -noout -modulus | openssl md5
openssl rsa -in naswii-signed-key.pem -noout -modulus | openssl md5
```

The two hashes should match.

Check the signed certificate subject and issuer:

```powershell
openssl x509 -in naswii-signed-cert.pem -noout -subject -issuer -serial
```

Expected shape:

```text
subject=... CN=naswii.nintendowifi.net
issuer=... CN=Wii NWC Prod 1 ...
```

## Remote HTTPS

For browser-facing HTTPS on your own domain, use a normal publicly trusted certificate for `fullchain.pem` and `privkey.pem`, such as one from Let's Encrypt.

Example with Certbot on Linux:

```bash
sudo certbot certonly --standalone -d wfc.example.com
sudo cp /etc/letsencrypt/live/wfc.example.com/fullchain.pem ./fullchain.pem
sudo cp /etc/letsencrypt/live/wfc.example.com/privkey.pem ./privkey.pem
sudo chown "$USER":"$USER" fullchain.pem privkey.pem
```

For clients connecting to Nintendo WFC hostnames, routing still matters. The game asks for `naswii.nintendowifi.net`, not your domain, unless a DNS server, hosts file, or payload redirects it.

## Common Errors

`open privkey.pem: The system cannot find the file specified` means `certPath` or `keyPath` points to a missing file.

`Wii HTTPS exploit unavailable` means its configured certificate set is missing or invalid. The server logs the error and keeps the HTTP web panel available. Install matching files, then restart.

`tls: client offered only unsupported versions: [302 301]` means the server was built without legacy TLS support. Rebuild from current source.

`20100` after `QR2: AVAILABLE` but before NAS login usually means Dolphin did not complete the HTTPS NAS step. Check hosts entries, Dolphin SSL verification settings, and certificate files.
