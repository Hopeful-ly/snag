# Releasing Snag

## Making a release

Push a version tag:

    git tag v1.2.0 && git push origin v1.2.0

The **Release** workflow (`.github/workflows/release.yml`) builds everything, installs the `.deb` and
the Windows installer on clean machines to check them, and makes a **draft** GitHub release with the
files. Look it over, then publish it. Then run the **Feed** workflow (Actions > Feed > Run
workflow), so installed copies of Snag hear about it. To try a packaging change without a
release, run the Release workflow by hand: Actions > Release > Run workflow.

CI checks every pull request on Linux and Windows, and pushes to `main` on Linux; run it by hand
for macOS too.

Releases are numbered by their tag; untagged builds are 0.0.0. Release builds also know they are
installed (`version.Packaged`), so each start points the browsers at the installed Snag. Development
builds never do that by themselves.

| File | What it is | Made by |
|---|---|---|
| `Snag-VERSION-windows-amd64-setup.exe` | Per-user installer: no administrator needed. Brings yt-dlp and an LGPL FFmpeg, and the WebView2 runtime if it's missing. | `task package:windows` (on Linux, needs `nsis`) |
| `snag_VERSION_amd64.deb`, `.rpm`, `.pkg.tar.zst` | Linux packages; FFmpeg comes from the system. | `task package:linux` (needs `nfpm`) |
| `Snag-VERSION-macos.dmg` | Snag.app for Apple silicon and Intel. | `task package:macos` (on a Mac) |
| `snag-extension-VERSION.zip` | The extension for Chromium browsers, to load unpacked. The app installs the same thing into its own folder. | `task package:extension` |
| `snag-firefox-VERSION.xpi` | The Firefox extension, signed by Mozilla (only with AMO keys, see below). | `task package:extension` |
| `SHA256SUMS` | Every file's SHA-256, so a download can be checked. | the Release workflow |

Everything lands in `dist/`.

## What needs your accounts

These all work without the accounts, but with warnings or missing pieces. Each one is a secret or
setting in the GitHub repository (Settings → Secrets and variables → Actions).

### Firefox signing (Mozilla Add-ons)

Firefox only installs signed extensions. Mozilla signs add-ons that aren't in its store too (the
"unlisted" channel), after an automatic review.

1. Make an account at addons.mozilla.org and create API keys (Tools → Manage API Keys).
2. Add them as the secrets `AMO_JWT_ISSUER` and `AMO_JWT_SECRET`. Tagged releases then come with a
   signed `snag-firefox-VERSION.xpi`.
3. The extension's id is `snag@snagdl.com`, in `extension/manifests/firefox.json` and
   `internal/nativehost/nativehost.go` (a test checks they agree). Mozilla ties the add-on to it
   for good: never change it.
4. The variable `SNAG_FIREFOX_UPDATE_URL` (`https://updates.snagdl.com/firefox/updates.json`) is
   built into the add-on as where Firefox looks for newer versions, once a day.
5. The **Feed** workflow publishes each release's signed add-on to the feed's bucket:
   `firefox/snag-firefox-VERSION.xpi`, `firefox/snag.xpi` (always the latest) and
   `firefox/updates.json`. Firefox only offers to install an add-on served as
   `application/x-xpinstall`, which GitHub's downloads aren't. `snagdl.com/firefox` redirects to
   `snag.xpi`, and the app's "Connect your browser" screen opens it.

### Windows code signing

Without a signature, SmartScreen warns about the installer ("Windows protected your PC"). Snag
signs `snag.exe`, `snag-host.exe` and the installer with
[Azure Trusted Signing](https://learn.microsoft.com/azure/trusted-signing/), through
[jsign](https://ebourg.github.io/jsign/), which runs on Linux, where the installer is made
(`packaging/windows/sign.sh`, called by `task package:windows`). Without the secrets they stay
unsigned.

1. Check you can use it: Trusted Signing verifies who you are, and takes individuals only from
   some countries. If it won't take you, a certificate from a certificate authority with a cloud
   key (SSL.com eSigner, Certum SimplySign and others) works too; jsign supports those, with
   another `--storetype` in `sign.sh`.
2. In Azure, make a Trusted Signing account and a certificate profile ("Public Trust"), and an app
   registration with a client secret, given the role "Trusted Signing Certificate Profile Signer"
   on the account.
3. Add the secrets `AZURE_TENANT_ID`, `AZURE_CLIENT_ID` and `AZURE_CLIENT_SECRET`, and the variables
   `TRUSTED_SIGNING_ENDPOINT` (the account's region, such as `weu.codesigning.azure.net`),
   `TRUSTED_SIGNING_ACCOUNT` and `TRUSTED_SIGNING_PROFILE`.

The uninstaller inside the installer stays unsigned for now; Windows only checks it when it runs.

### macOS signing and notarization

The app is signed ad hoc, so Gatekeeper asks the first time ("right-click → Open"). With an Apple
Developer ID, the Release workflow signs the app and the disk image with it
(`packaging/macos/make-app.sh`), and sends the disk image to Apple to notarize, so Gatekeeper
opens it without asking.

1. Join the Apple Developer Program, and make a "Developer ID Application" certificate. Export it
   from Keychain Access as a `.p12` with a password.
2. Add the secrets `MACOS_CERT_P12` (the file in base64: `base64 -i cert.p12 | pbcopy`) and
   `MACOS_CERT_PASSWORD`, and the variable `MACOS_SIGN_IDENTITY`, the certificate's name, such as
   `Developer ID Application: Your Name (TEAMID)`.
3. For notarizing, make an App Store Connect API key (Users and Access → Integrations, the
   "Developer" role) and add the secrets `APPLE_API_KEY` (the `.p8` file's contents),
   `APPLE_API_KEY_ID` and `APPLE_API_ISSUER`.

Not tried with real accounts yet: the first signed release may need fixes, such as entitlements
for the bundled yt-dlp under the hardened runtime.

### Snag's feed

Installed Snag reads a small file on Snag's website once a day (`internal/feed`): the latest
release with the SHA-256 of each of its installers, and the yt-dlp to use. It is signed with an
Ed25519 key whose public half is built into the app (`feed.PublicKey`), so whoever serves it
can't change what Snag trusts, and Snag never takes a feed older than one it already has.

Snag downloads the installer for how it was installed and checks it against the feed
(`internal/update`), then offers "Restart to update" in About and the tray, with one
notification. Restarting installs it: the Windows installer runs quietly once Snag has quit,
macOS swaps in the disk image's Snag.app, and Linux installs the package through `pkexec`, which
asks for the password. The new Snag then starts. Copies Snag can't replace, such as a
development build, link to the release page instead.

The **Feed** workflow (`.github/workflows/feed.yml`) signs it with `cmd/sign-feed` and puts it
in a Cloudflare R2 bucket the website serves. Its inputs: the latest Snag (the latest tag if left
empty, and none before the first release; its release's `SHA256SUMS` names the installers) and the yt-dlp to use. Name a yt-dlp version
(`2026.10.10`, or `nightly@…`) to hold every Snag at it, older or newer, when yt-dlp's latest
release broke a site; leave it empty to follow yt-dlp's latest stable release again.

1. **The key.** Its private half, the secret `SNAG_FEED_KEY` (base64), is the only thing that can
   make a feed Snag believes. Keep a copy somewhere safe, such as a password manager: losing it
   means a new key, which only reaches users with an app update; anyone holding it can tell every
   Snag which yt-dlp to run.
2. **The storage.** Make an R2 bucket (`snag-updates`), give it a custom domain
   (`updates.snagdl.com`), and an R2 API token that can write to it. Add the secrets
   `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID` and `R2_SECRET_ACCESS_KEY`, and the variable `FEED_BUCKET`.
3. **The address.** Set the variable `SNAG_FEED_URL` to the feed's address
   (`https://updates.snagdl.com/feed.json`): release builds look there. Without it they never
   look; development builds never do.

### Linux packages

Set a real contact in `maintainer:` in `packaging/linux/nfpm.yaml`. For apt and dnf repositories
later, the packages would also need signing with a GPG key.

## Bundled software

- **yt-dlp** is public domain. Installed Snag copies it to its data folder and keeps it up to date.
- **FFmpeg** (Windows): an LGPL shared build from BtbN, pinned in `internal/tools/ffmpeg.go`. The
  LGPL asks you to say Snag uses it and to offer its source: link the exact build's source on the
  website and mention it in About. Linux packages use the system's FFmpeg. macOS has no LGPL
  build to ship yet, so Snag uses an installed one (Homebrew) if there is one.
