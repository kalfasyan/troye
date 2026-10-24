# Troye on a Synology NAS

This guide sets Troye up on a Synology DS220+ for a household: the app on the NAS,
both phones on `https://troye.<name>.synology.me` at home and away, and the GPU PC
analysing meal photos when it is on. It takes about half an hour.

```
 phones ──HTTPS──▶ router :443 ──▶ DSM reverse proxy ──▶ Troye on 127.0.0.1:8092 ── /volume1/docker/troye/pb_data
                                                              ▲         │
              GPU PC: analyser + Ollama ──HTTPS (home network)┘         └── Wake-on-LAN ──▶ GPU PC
```

`<name>` is your Synology DDNS name, the same one the other apps on the NAS use.
The examples use port 8092; pick any port that no other app on the NAS uses.

## Before you start

- DSM 7.2 or later, with **Container Manager** installed from Package Center.
- A DDNS name under Control Panel > External Access > DDNS, with a Let's Encrypt
  wildcard certificate for `*.<name>.synology.me` (Control Panel > Security >
  Certificate). If the other apps on the NAS already use `something.<name>.synology.me`,
  you have both.
- Port 443 on the router forwarded to the NAS.
- SSH on: Control Panel > Terminal & SNMP > Enable SSH service.

## 1. Install the app

Sign in to the NAS from a computer: `ssh <your DSM user>@<NAS address>`. Then run:

```sh
curl -fsSL https://raw.githubusercontent.com/selfhostit/troye/main/deploy/install.sh |
  sudo PORT=8092 sh -s /volume1/docker/troye
```

Press Enter when it asks for a Tailscale key; the reverse proxy below replaces it.
The script creates `/volume1/docker/troye` with the settings in `.env` and the
database in `pb_data`, owned by your DSM user, and starts the app. It ends with
`troye is running`.

## 2. Settings

Open the settings, for example with `sudo vi /volume1/docker/troye/.env` (or with
the Text Editor package through File Station), and set:

```sh
TZ=Europe/Brussels
# the app on the NAS's own network, so Wake PC reaches the GPU PC
COMPOSE_FILE=docker-compose.yml:docker-compose.host.yml
# the GPU PC's wired network card, and your network's broadcast address
TROYE_WAKE_MAC=aa:bb:cc:dd:ee:ff
TROYE_WAKE_ADDR=192.168.1.255:9
```

The broadcast address is the NAS's address with the last number set to 255 when
the subnet mask is `255.255.255.0` (Control Panel > Network > Network Interface).
Leave the wake lines empty until you set up the PC; you can add them later.

Apply the settings:

```sh
cd /volume1/docker/troye && sudo docker compose up -d
```

## 3. The web address

1. Control Panel > Login Portal > Advanced > **Reverse Proxy** > Create:
   - Name: `troye`
   - Source: protocol **HTTPS**, host name `troye.<name>.synology.me`, port **443**,
     and tick **Enable HSTS**.
   - Destination: protocol **HTTP**, host name `localhost`, port **8092**.
2. Control Panel > Security > Certificate > **Settings**: set the wildcard certificate
   for `troye.<name>.synology.me`.
3. Open `https://troye.<name>.synology.me` on a computer. You should see the setup
   page.

Push notifications and installing the app on a phone need this HTTPS address.

## 4. Your accounts

1. On the setup page, create your own account. The first account is the owner, and
   the same email and password open the admin page at `/_/`. Use a strong password,
   since the admin page is reachable from the internet too.
2. Add your wife's account: open `https://troye.<name>.synology.me/_/`, go to
   Collections > **users** > New record, and fill in her email, name and a starting
   password (twice), with **role** set to `member`. Save.
3. Nobody else can create an account: after the owner, only the owner can add people.

Each account has its own plan, diary, photos and weigh-ins, visible only to that
person.

## 5. On each phone (Android)

1. Open `https://troye.<name>.synology.me` in **Chrome** and sign in.
2. Chrome's menu (⋮) > **Install app** (or **Add to home screen**). Troye now opens
   from its own icon, full screen.
3. Answer the onboarding questions. They set the daily calorie budget, the macros
   and the fasting window.
4. In the **Me** tab, turn on notifications and allow them. That is how the phone
   hears that photos are ready to check, and gets the Monday summary.

## 6. The GPU PC

Follow [The photo analyser](deploy.md#the-photo-analyser-gpu-pc) in the deploy
guide, with two things specific to the NAS:

- Point the analyser at `TROYE_URL=https://troye.<name>.synology.me`. If the PC
  can't reach that address from home (some routers don't loop back to the NAS), make
  the name point at the NAS directly: add `--add-host
  troye.<name>.synology.me:<NAS address>` to the `docker run` line, or add a line
  `<NAS address> troye.<name>.synology.me` to the PC's hosts file when it runs
  without Docker.
- For **Wake PC**, set `TROYE_WAKE_MAC` and `TROYE_WAKE_ADDR` as in step 2 and
  turn on Wake-on-LAN on the PC
  ([Waking the PC](deploy.md#waking-the-pc-from-the-phone)). The PC needs a cable.

Then in the app, take a photo of a meal with the PC off. The diary shows the photo
waiting and a **Wake PC** button. Tap it: the PC starts, the analyser picks the
photo up, and the phone gets a notification when it is ready to check.

## Backups

The app backs up its database and photos every night at 03:00 and keeps the last
7, in `pb_data/backups` on the same disk. To keep a copy elsewhere, add
`docker/troye` to a **Hyper Backup** task, to a USB disk or another NAS. To restore
a nightly backup, use the admin page: Settings > Backups.

## Updates

```sh
cd /volume1/docker/troye && sudo docker compose pull && sudo docker compose up -d
```

Running the install script again does the same and also refreshes the compose
files. Your settings and data stay.

## When something is off

- **See the logs:** Container Manager > Container > the `troye-app-1` container >
  Log, or `sudo docker compose logs app` in the folder.
- **The page doesn't load:** check that `curl http://127.0.0.1:8092/api/health`
  answers on the NAS, then the reverse proxy rule and the certificate.
- **The install stops with a port error:** another app has the port. Set `PORT` in
  `.env` to a free one, run `sudo docker compose up -d`, and change the reverse
  proxy's destination port to match.
- **No notifications:** open the app from the HTTPS address (not the NAS's IP),
  turn them on in the Me tab, and check that Chrome may show notifications in
  Android's settings.
- **Wake PC does nothing:** check that `COMPOSE_FILE` includes
  `docker-compose.host.yml`, the MAC address and the broadcast address, and that the
  PC wakes from another device's Wake-on-LAN app.
