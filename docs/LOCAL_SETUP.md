# Local Dolphin Setup

This setup runs the WFC server and Dolphin on the same Windows PC. It uses Docker for PostgreSQL, Windows `hosts` entries for Nintendo domains, and HTTPS enabled locally.

## Working Mode

Use this mode for local Dolphin:

```yaml
allowNoSSLCheatClients: true
```

The name is historical. In this local setup it means the server treats the Dolphin client as already redirected/patched and does not inject the SBCM payload when you click Worldwide. Keep the Mario Kart Wii NoSSL cheat disabled.

Tradeoff: normal login and matchmaking work, but payload-only features such as the custom login popup are not available.

## 1. Start PostgreSQL

From the repository root:

```powershell
docker compose up -d postgres
```

The compose file creates:

```text
database: wwfc
username: wwfc_user
password: wwfc_password
host:     127.0.0.1
port:     5432
```

`schema.sql` is imported automatically the first time the Docker volume is created.

To recreate the database from scratch:

```powershell
docker compose down -v
docker compose up -d postgres
```

This deletes the database volume.

## 2. Configure `config.yml`

> [!TIP]
> Without a `config.yml`, the server starts a setup wizard in the browser that writes the file for you and creates the owner account; see [First-run setup](../README.md#first-run-setup). With Docker Desktop running, its Database step can also create PostgreSQL for you, so the `docker compose up -d postgres` step above becomes optional. Every option can be changed later in **Panel → Server**. The steps below describe editing the file by hand.

Copy the YAML example if needed:

```powershell
Copy-Item config_example.yml config.yml
```

`config.yml` is the default configuration file. `config.xml` is only read as a legacy fallback when no `config.yml`/`config.yaml` exists.

Use these database values:

```yaml
databaseAddress: 127.0.0.1
databaseName: wwfc
username: wwfc_user
password: wwfc_password
```

Use loopback addresses for Dolphin on the same PC:

```yaml
gsAddress: 127.0.0.1
nasAddress: 127.0.0.1
nasAddressHttps: 127.0.0.1
```

Enable HTTPS:

```yaml
enableHttps: true
certPath: fullchain.pem
keyPath: privkey.pem
```

For the local Dolphin mode that worked in testing:

```yaml
payloadServerAddress: ""
allowNoSSLCheatClients: true
```

If `allowNoSSLCheatClients` is `false`, the server will inject `payload/sbcm/payload.RMCP.bin` when Worldwide starts. That is only for the DNS payload flow and can disconnect/crash this local Dolphin setup.

## 3. Generate HTTPS Certificates

Follow:

```text
docs/HTTPS_CERTIFICATES.md
```

For this local setup you need at least:

```text
fullchain.pem
privkey.pem
```

If `enableHttpsExploitWii` is enabled, also generate:

```text
naswii-chain.der
naswii-signed-key.pem
```

Then configure:

```yaml
enableHttpsExploitWii: true
certDerPathWii: naswii-chain.der
keyPathWii: naswii-signed-key.pem
```

## 4. Configure Dolphin

Dolphin may use either the AppData config or a portable config. Check both if needed:

```text
%APPDATA%\Dolphin Emulator\Config\Dolphin.ini
D:\dolphin-master-5.0-13603-x64\Dolphin Emulator\Config\Dolphin.ini
```

Add or update:

```ini
[Network]
SSLVerifyCertificates = False
SSLVerifyCert = False
```

Make sure:

```text
NoSSL cheat: disabled
Enable Cheats: disabled, unless you need unrelated cheats
```

If Dolphin says the emulated Wii is outdated, run:

```text
Tools -> Perform Online System Update -> Current Region
```

## 5. Add Windows Hosts Entries

Edit this file as Administrator:

```text
C:\Windows\System32\drivers\etc\hosts
```

For local Dolphin, map Nintendo WFC hosts to `127.0.0.1`:

```text
127.0.0.1 conntest.nintendowifi.net
127.0.0.1 naswii.nintendowifi.net
127.0.0.1 nas.nintendowifi.net
127.0.0.1 dls1.nintendowifi.net
127.0.0.1 gpcm.gs.nintendowifi.net
127.0.0.1 gpsp.gs.nintendowifi.net
127.0.0.1 gamestats.gs.nintendowifi.net
127.0.0.1 gamestats2.gs.nintendowifi.net
127.0.0.1 sake.gs.nintendowifi.net
127.0.0.1 mariokartwii.available.gs.nintendowifi.net
127.0.0.1 mariokartwii.master.gs.nintendowifi.net
127.0.0.1 mariokartwii.ms19.gs.nintendowifi.net
127.0.0.1 mariokartwii.gamestats.gs.nintendowifi.net
127.0.0.1 mariokartwii.gamestats2.gs.nintendowifi.net
127.0.0.1 mariokartwii.sake.gs.nintendowifi.net
127.0.0.1 mariokartwii.natneg1.gs.nintendowifi.net
127.0.0.1 mariokartwii.natneg2.gs.nintendowifi.net
127.0.0.1 mariokartwii.natneg3.gs.nintendowifi.net
```

Flush DNS after editing:

```powershell
ipconfig /flushdns
```

Check a few names:

```powershell
Resolve-DnsName naswii.nintendowifi.net
Resolve-DnsName gpcm.gs.nintendowifi.net
Resolve-DnsName mariokartwii.available.gs.nintendowifi.net
```

They should resolve to `127.0.0.1`.

### Generated Gecko patch and payload features

The generated Gecko code for domain `localhost` redirects the game to `nas.localhost` and `gs.localhost`. Windows may not resolve these subdomains automatically. Add these names to the same `hosts` file before enabling the code:

```text
127.0.0.1 nas.localhost naswii.localhost gs.localhost
127.0.0.1 gpcm.gs.localhost gpsp.gs.localhost gamestats.gs.localhost gamestats2.gs.localhost
127.0.0.1 mariokartwii.available.gs.localhost mariokartwii.master.gs.localhost mariokartwii.ms19.gs.localhost
127.0.0.1 mariokartwii.gamestats.gs.localhost mariokartwii.gamestats2.gs.localhost mariokartwii.sake.gs.localhost
127.0.0.1 mariokartwii.natneg1.gs.localhost mariokartwii.natneg2.gs.localhost mariokartwii.natneg3.gs.localhost
```

Run `ipconfig /flushdns` and confirm `Resolve-DnsName nas.localhost` and `Resolve-DnsName gs.localhost` return `127.0.0.1`. For the PAL disc build, add the current patcher's `patch/build/RMCPD00.txt` as an enabled Gecko code in Dolphin. `RMCPN0001.txt` targets a different game version; do not enable both. Leave NoSSL disabled. Copy the same patcher build's `dist/` to the server's `payload/` and copy `dist/stage1.bin` again as `payload/stage1v1.bin`: the Gecko code downloads `/w1`. Verify `http://nas.localhost/w1` returns HTTP 200 before starting the game.

Once the game connects, **Live** in the web panel should mark the player **Payload**. This is the check for payload-only features such as the custom messages and the Mario Kart Wii race-end action.

## 6. Build and Run

Run PowerShell as Administrator if you bind to ports `80` and `443`:

```powershell
go build -o dwc-server-advanced.exe
.\dwc-server-advanced.exe
```

The public web panel is:

```text
http://127.0.0.1/
```

The moderator/admin login is:

```text
http://127.0.0.1/login
```

The first account created is the owner.

## 7. Expected Logs

Successful local Dolphin login:

```text
I[QR2]: Command: AVAILABLE
N[NAS:...]: Login (Wii) ...
N[GPCM:...]: Connection established
I[GPCM:...]: Command: login
N[DATABASE]: Log in GPCM user ...
N[QR2:...]: Opened session with PID ...
```

Successful Worldwide without payload injection should not show:

```text
Sending SBCM exploit to DNS patcher client
```

If you see that line in local Dolphin mode, set:

```yaml
allowNoSSLCheatClients: true
```

and restart the server.

## Troubleshooting

`20100` with only `QR2: AVAILABLE` means Dolphin reached the availability check but did not complete NAS/GPCM login. Check hosts entries, HTTPS cert files, and Dolphin SSL verification settings.

`tls: client offered only unsupported versions: [302 301]` means the server build does not allow legacy TLS 1.1/TLS 1.0. Rebuild from the current source.

`Salt hash mismatch` during `/payload?...` means the payload URL hash or payload files do not match. Use the current source and make sure all payload files come from the same patcher build.

Login messages use the payload MOTD path when the client has payload features. Vanilla Mario Kart Wii does not display arbitrary native MOTD text; its native `bm=1` GPCM messages are matchmaking commands, not text popups.
