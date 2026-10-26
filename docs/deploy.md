# Deploy

A troye app runs as one container on any Linux machine with Docker: a NAS, a
Raspberry Pi, a small server. Workers run next to it or on other machines.
Tailscale gives your devices an `https://` address for it without opening
anything to the internet.

```
 phone / laptop ──Tailscale──▶ tailscale sidecar ──▶ app (PocketBase + web app) ── pb_data/
                                                       ▲
                     worker on the same machine ───────┤  (records through the API)
                     worker on a Pi ──Tailscale────────┘
```

Every push to `main` publishes two images for amd64, arm64 and armv7:
`ghcr.io/selfhostit/troye` (the app) and `ghcr.io/selfhostit/troye-worker` (the
Python workers). A tag like `v1.2.0` publishes `:v1.2.0` of both.

## Install

On the machine, as a user that can run `docker`:

```sh
curl -fsSL https://raw.githubusercontent.com/selfhostit/troye/main/deploy/install.sh | sh
```

This creates `~/troye` with the compose files, a `.env` and a `pb_data` folder
for the database, asks for a Tailscale auth key (press Enter to skip, or set an
empty `TS_AUTHKEY=` to skip the question), and starts the app. Pass a folder to
install elsewhere: `... | sh -s /srv/troye`.

Then open the app and create your account; the first person becomes the owner.
Without Tailscale it is only reachable on the machine itself, at
http://localhost:8090 (the port is bound to `127.0.0.1` on purpose).

Settings live in `.env`: time zone, port, Tailscale, the worker account and the
image version. Change them and run `docker compose up -d` in the folder (or the
install script again).

## Remote access with Tailscale

1. In the [Tailscale admin console](https://login.tailscale.com/admin/dns), turn on
   **MagicDNS** and **HTTPS Certificates**.
2. Create an [auth key](https://login.tailscale.com/admin/settings/keys) and paste it
   when the install script asks. It is only used for the first sign-in; the
   sidecar keeps its login in `tailscale/`.
3. The app is now at `https://troye.<your-tailnet>.ts.net` (the script prints it).
   The first visit can take a few seconds while the certificate is issued.

On a phone, install the Tailscale app, open that address and add it to the home
screen. Push notifications need this HTTPS address.

To add Tailscale later, set `TS_AUTHKEY` and uncomment the `COMPOSE_FILE` line in
`.env`, then run `docker compose up -d`. `TS_HOSTNAME` sets the name.

## Updates and versions

```sh
cd ~/troye && docker compose pull && docker compose up -d
```

Running the install script again does the same and also refreshes the compose
files. To stay on a release, or go back to one, set `TROYE_IMAGE` (and
`TROYE_WORKER_IMAGE`) in `.env` to a tag, like `ghcr.io/selfhostit/troye:v1.2.0`.
Database migrations run on start, so take a backup before going back across one.

## Backups and restore

The app backs up the database and uploads every night at 03:00 and keeps the
last 7, in `pb_data/backups`. A backup on the same disk doesn't survive the disk,
so copy `pb_data` somewhere else too (Hyper Backup on a Synology, `rsync`, or S3
storage set in the admin UI under Settings > Backups).

To restore, open the admin UI (`/_/`), go to Settings > Backups and restore one.
To move to another machine, stop the app, copy the whole folder, and start it
there.

## Workers

### Next to the app

1. In the admin UI, open the **workers** collection and add one with a name and a
   password.
2. In `.env`, set `TROYE_WORKER` and `TROYE_WORKER_PASSWORD` and uncomment
   `COMPOSE_PROFILES=worker`.
3. Run `docker compose up -d`.

It runs `workers/worker.py`, the starting point for your own worker. To run another
one, set `TROYE_WORKER_SCRIPT` in `.env`, for example to `camera.py`. Every `.py`
file in `workers/` and every dependency you add with `uv add` is in the worker image.

### On another machine

Join the machine to your tailnet, then point the worker at the app's tailnet
address:

```sh
docker run -d --name worker --restart unless-stopped \
  -e TROYE_URL=https://troye.<your-tailnet>.ts.net \
  -e TROYE_WORKER=camera -e TROYE_WORKER_PASSWORD=... -e TZ=Europe/Athens \
  ghcr.io/selfhostit/troye-worker python camera.py
```

## The photo analyser (GPU PC)

Meal photos wait on the app until the GPU PC analyses them with a vision model in
[Ollama](https://ollama.com). Nothing leaves your home network.

1. On the PC, install Ollama (it uses the NVIDIA card through CUDA, or the AMD card
   through ROCm) and download the model: `ollama pull qwen2.5vl:7b`. It needs about
   6 GB of graphics memory, so the RTX 3060 (12 GB) and the RX 7900 XT (20 GB) both fit.
2. In the admin UI, add a worker named `analyser` with a password.
3. Run the analyser on the PC, so it starts with it:
   ```sh
   docker run -d --name analyser --restart unless-stopped --network host \
     -e TROYE_URL=https://troye.<name>.synology.me \
     -e TROYE_WORKER=analyser -e TROYE_WORKER_PASSWORD=... \
     -e OLLAMA_URL=http://127.0.0.1:11434 -e TZ=Europe/Brussels \
     ghcr.io/selfhostit/troye-worker python analyser.py
   ```
   Without Docker: `cd workers && uv run python analyser.py` with the same variables.
   `TROYE_MODEL` picks another Ollama vision model.

The analyser asks for photos every 20 seconds, which is also how the app knows the
PC is on. Each person gets a notification as their photos are ready to check.

### Picking the model

Once you have logged some meals from photos, score other vision models on them:

```sh
cd workers
ollama pull gemma3:12b
TROYE_URL=... TROYE_WORKER=analyser TROYE_WORKER_PASSWORD=... \
  uv run python evaluate.py qwen2.5vl:7b gemma3:12b
```

It prints how far off each model's calories and portions were and how many of the
logged foods it found, next to the drafts you actually saw. It writes nothing. Set
`TROYE_MODEL` to the winner. The Me tab shows the same scores for the model in use.

### Waking the PC from the phone

When photos wait and the PC is off, the diary shows **Wake PC**. It makes the app
send a Wake-on-LAN packet:

1. Turn on Wake-on-LAN in the PC's BIOS and for its network card (in Windows, the
   card's Power Management settings; in Linux, `ethtool -s <card> wol g`). The PC
   needs a wired connection.
2. In `.env`, set `TROYE_WAKE_MAC` to the PC's MAC address, and `TROYE_WAKE_ADDR` to
   your network's broadcast address with port 9, such as `192.168.1.255:9`.

3. Run the app on the host network, since a container on Docker's own network can't
   send a broadcast to the home network: uncomment
   `COMPOSE_FILE=docker-compose.yml:docker-compose.host.yml` in `.env` and run
   `docker compose up -d`. It still listens on `127.0.0.1` at `PORT` only. This
   replaces the Tailscale sidecar, so use a reverse proxy instead.

## Synology NAS

[synology.md](synology.md) walks through the whole setup on a Synology, from the
install to both phones and the GPU PC. In short:

Needs DSM 7.2 or later with **Container Manager** installed.

1. In Control Panel > Terminal & SNMP, enable SSH, then sign in with
   `ssh you@nas.local`.
2. Install into a shared folder, with `sudo` because Docker on DSM needs it:
   ```sh
   curl -fsSL https://raw.githubusercontent.com/selfhostit/troye/main/deploy/install.sh | sudo sh -s /volume1/docker/troye
   ```
   The data stays owned by your DSM user (`PUID` and `PGID` in `.env`), so you can
   see and back it up in File Station.
3. The containers show up in Container Manager, where you can see their logs and
   restart them. Update from SSH as above.

To serve the app on the internet instead of only your tailnet, add a rule under
Control Panel > Login Portal > Advanced > Reverse Proxy from your HTTPS host name
to `localhost:8090`. Never do this for apps that show cameras.

## Raspberry Pi

Use a Pi 4 or 5 with the 64-bit Raspberry Pi OS (Lite is enough).

```sh
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER   # then sign out and back in
```

Then run the install script as above to host the app on the Pi, or follow
[On another machine](#on-another-machine) to use the Pi only for workers, with
Tailscale installed on the Pi itself
(`curl -fsSL https://tailscale.com/install.sh | sh && sudo tailscale up`).

Hardware a worker needs, like a Hailo accelerator or a camera, is passed to its
container with `devices:` in compose (or `--device /dev/hailo0` with `docker run`).
Large services such as Frigate run as their own containers next to the worker.

## Check a deploy

`make smoke` builds both images, deploys them with `docker-compose.yml` in a
scratch folder, signs up an owner, runs one round of the worker and checks that
it reported in. CI runs the same script on every pull request.
