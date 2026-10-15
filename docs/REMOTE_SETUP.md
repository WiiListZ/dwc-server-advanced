# Remote Server Setup

This setup is for a VPS or dedicated server. It covers the server process, PostgreSQL, DNS/host routing, HTTPS, and payload mode.

## 1. Choose a Client Routing Method

Mario Kart Wii still asks for Nintendo WFC hostnames such as:

```text
naswii.nintendowifi.net
gpcm.gs.nintendowifi.net
mariokartwii.available.gs.nintendowifi.net
```

Your server must receive those connections.

For Dolphin clients, either:

```text
Edit the client's hosts file and point the Nintendo hostnames to your VPS IP.
```

For real Wii clients, use a DNS server that returns your VPS IP for the Nintendo WFC hostnames.

Your own domain, such as `wfc.example.com`, is still useful for the web panel, payload builds, documentation, and DNS infrastructure, but the unmodified game does not automatically use it unless DNS/payloads redirect it.

## 2. DNS

Create records for your own server domain:

```text
wfc.example.com   -> your server public IP
*.wfc.example.com -> your server public IP
```

If your DNS server supports overrides for Nintendo domains, map the required WFC hostnames to the same public IP.

For a Dolphin client using a hosts file, use the same host list from `docs/LOCAL_SETUP.md`, but replace `127.0.0.1` with the VPS public IP.

## 3. Start PostgreSQL

The compose file can run PostgreSQL on the remote host:

```bash
docker compose up -d postgres
```

By default PostgreSQL is bound to `127.0.0.1:5432`, so it is reachable by the server process on the same machine but not exposed publicly.

For production, change the password in `.env` or in your deployment environment:

```text
WWFC_POSTGRES_PASSWORD=use-a-long-random-password
```

Use the same password in `config.yml`.

## 4. Configure `config.yml`

> [!TIP]
> Without a `config.yml`, the server starts a setup wizard in the browser that writes the file for you and creates the owner account; see [First-run setup](../README.md#first-run-setup). Every option can be changed later in **Panel → Server**. The steps below describe editing the file by hand.

Bind game and NAS services to all interfaces:

```yaml
gsAddress: 0.0.0.0
nasAddress: 0.0.0.0
nasAddressHttps: 0.0.0.0
```

Use local PostgreSQL:

```yaml
databaseAddress: 127.0.0.1
databaseName: wwfc
username: wwfc_user
password: use-a-long-random-password
```

Enable HTTPS:

```yaml
enableHttps: true
certPath: fullchain.pem
keyPath: privkey.pem
```

For Wii HTTPS exploit support:

```yaml
enableHttpsExploitWii: true
certDerPathWii: naswii-chain.der
keyPathWii: naswii-signed-key.pem
```

Generate these files with `docs/HTTPS_CERTIFICATES.md`.

## 5. Pick Payload Policy

For a controlled Dolphin server where clients use hosts-file redirects and no payload features:

```yaml
allowNoSSLCheatClients: true
```

This skips SBCM payload injection.

For a public server that expects the DNS payload flow:

```yaml
allowNoSSLCheatClients: false
payloadServerAddress: ""
```

Then generate and install payloads as described in `docs/PAYLOADS.md`.

Do not mix these modes for the same client. If the game is already redirected/patched and the server injects `payload/sbcm/payload.RMCP.bin`, Mario Kart Wii can disconnect when Worldwide starts.

## 6. Firewall

Open these ports to clients:

```text
TCP 80      NAS HTTP and web panel
TCP 443     NAS HTTPS
TCP 28910   serverbrowser
TCP 29900   gpcm
TCP 29901   gpsp
TCP 29920   gamestats
UDP 27900   QR2
UDP 27901   NATNEG
```

Keep these local only:

```text
TCP 5432    PostgreSQL
TCP 29998   frontend RPC
TCP 29999   backend RPC
```

## 7. Build and Run

```bash
go build -o dwc-server-advanced
./dwc-server-advanced
```

Run it from the repository root so relative paths resolve:

```text
config.yml
payload/
state/
logs/
fullchain.pem
privkey.pem
naswii-chain.der
naswii-signed-key.pem
```

For long-running Linux hosting, use systemd, tmux, screen, or your process supervisor.

## 8. Checks

From the server:

```bash
curl -H "Host: naswii.nintendowifi.net" http://127.0.0.1/w0
curl -k --resolve naswii.nintendowifi.net:443:127.0.0.1 https://naswii.nintendowifi.net/ac
```

From a client machine:

```bash
curl http://wfc.example.com/
```

The public homepage is:

```text
http://wfc.example.com/
```

The admin login is:

```text
http://wfc.example.com/login
```

The first account created is the owner.

## Disabling Games

Disabled games are stored in the `public.disabled_games` database table and managed from the moderation panel (**Games** section, `/panel`). The server rejects disabled games at the NAS login stage (so the custom error code shows on vanilla, DNS-patched, and payload-patched clients alike) and again at GPCM login as a backstop. Changes made in the panel apply immediately, without a restart. When a game is disabled (or an entry is edited), every player already online with that game is disconnected right away and sees the same error message.

Access requires the **Disable and re-enable games** right (`manage_games`). The owner always has it; moderators get it only when the owner grants it from the Accounts section.

Fields:

- **Game code** (`game_code_prefix`): the Nintendo game code sent at NAS login. Codes are 4 characters and the last one is the region (`RMCP` is Mario Kart Wii PAL). A code without the region letter applies to every region of that game: `RMC` blocks `RMCP`, `RMCE`, `RMCJ`, `RMCK` and any other region; `ABC` does the same for game `ABC`. A full 4-character code blocks only that region. In the panel, the **All regions** switch (on by default) turns a full code such as `RMCP` into `RMC` before saving. Used by NAS and by the disconnect of online players.
- **GameSpy Game Name** (`game_name`): GameSpy game name as it appears in [`common/game_list.tsv`](../common/game_list.tsv) column 2 (e.g. `mariokartwii`). Used by GPCM.
- **NAS Return Code** (`nas_return_code`): 3-digit NAS return code. Nintendo DWC maps this to `20XXX` on screen (e.g. `110` → `20110`). Defaults to `110`.
- **Payload Error Code** (`error_code`): error code displayed by the payload via `wl:err` during GPCM rejection. Only visible on clients with the payload injected. Defaults to `20000 + NAS return code`.
- **Message** (`message`): text shown by the payload (GPCM stage only). Line breaks are honored. `Error Code: %[1]d` is appended automatically.

At least one of game code prefix or game name is required. Each prefix and each game name can only be disabled once.

The panel API is `GET/POST/PATCH/DELETE /api/panel/disabled-games`. `GET /api/panel/game-list` returns the games from `game_list.tsv` for the game picker. Panel changes are logged as `disabled_game_added`, `disabled_game_updated` and `disabled_game_removed` events.

The table is read into memory at startup. If you edit it directly in PostgreSQL, restart the server to apply the change.

### Migrating from `disabled_games.json`

Older versions read `disabled_games.json` from the server root. On the first startup with this version, its entries are imported into the database and the file is renamed to `disabled_games.json.imported`. Entries whose prefix or game name already exist in the table are skipped.

## Expected Logs

Successful login:

```text
I[QR2]: Command: AVAILABLE
N[NAS:...]: Login (Wii) ...
N[GPCM:...]: Connection established
I[GPCM:...]: Command: login
N[DATABASE]: Log in GPCM user ...
```

Payload mode will additionally show:

```text
Sending SBCM exploit to DNS patcher client
Loaded SBCM exploit: payload\sbcm\payload.RMCP.bin
Command: CLIENT_EXPLOIT_ACK
```
